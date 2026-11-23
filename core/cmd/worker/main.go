// Command worker runs background jobs. Day 1 only connects to Postgres and waits;
// River job processing is added on Day 2.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/FinA1ee/dipwatch/core/internal/config"
	"github.com/FinA1ee/dipwatch/core/internal/db"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("worker failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	url, err := config.LoadDatabaseURL()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	slog.Info("worker ready")
	<-ctx.Done()
	slog.Info("worker shutting down")
	return nil
}
