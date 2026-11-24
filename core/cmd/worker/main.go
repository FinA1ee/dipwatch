// Command worker runs background jobs.
//
// Usage:
//
//	worker                       process jobs until stopped
//	worker refresh "RTX 5090"    create the product if needed, enqueue a refresh, and exit
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/FinA1ee/dipwatch/core/internal/config"
	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/jobs"
	"github.com/FinA1ee/dipwatch/core/internal/sources"
	"github.com/FinA1ee/dipwatch/core/internal/sources/kakaku"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// maxWorkers keeps concurrent source fetches low: the server has 2 vCPU and 4 GB (spec N1).
const maxWorkers = 2

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	var err error
	if len(os.Args) > 2 && os.Args[1] == "refresh" {
		err = enqueueRefresh(os.Args[2])
	} else {
		err = run()
	}
	if err != nil {
		slog.Error("worker failed", "err", err)
		os.Exit(1)
	}
}

func adapters() jobs.Adapters {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return jobs.Adapters{
		"kakaku": {"html": kakaku.New(httpClient, kakaku.DefaultBaseURL)},
	}
}

func open(ctx context.Context) (*pgxpool.Pool, error) {
	url, err := config.LoadDatabaseURL()
	if err != nil {
		return nil, err
	}
	return db.Open(ctx, url)
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := open(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.RefreshProductWorker{
		DB:       pool,
		Registry: sources.NewRegistry(),
		Adapters: adapters(),
	})
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:          map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: maxWorkers}},
		Workers:         workers,
		SoftStopTimeout: 20 * time.Second,
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	slog.Info("worker ready", "max_workers", maxWorkers)

	<-ctx.Done()
	slog.Info("worker shutting down")
	<-client.Stopped()
	return nil
}

// enqueueRefresh is a development helper until search (Day 4) creates products.
func enqueueRefresh(name string) error {
	if name == "" {
		return errors.New("product name is empty")
	}
	ctx := context.Background()
	pool, err := open(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	product, err := store.New(pool).UpsertProduct(ctx, store.UpsertProductParams{CanonicalName: name})
	if err != nil {
		return err
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		return err
	}
	res, err := client.Insert(ctx, jobs.RefreshProductArgs{ProductID: product.ID}, nil)
	if err != nil {
		return err
	}
	slog.Info("refresh enqueued", "product_id", product.ID, "job_id", res.Job.ID, "duplicate", res.UniqueSkippedAsDuplicate)
	return nil
}
