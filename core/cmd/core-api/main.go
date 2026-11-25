// Command core-api serves the Dipwatch core HTTP API.
//
// Usage:
//
//	core-api [serve]   run migrations (unless disabled) and serve HTTP
//	core-api migrate   apply pending migrations and exit
//	core-api openapi   print the OpenAPI contract as YAML
package main

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/FinA1ee/dipwatch/core/internal/guard"
	httpapi "github.com/FinA1ee/dipwatch/core/internal/http"
	"github.com/FinA1ee/dipwatch/core/internal/llm"
	"github.com/FinA1ee/dipwatch/core/internal/runs"
	"github.com/FinA1ee/dipwatch/core/internal/search"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "migrate":
		err = migrate()
	case "openapi":
		err = printOpenAPI()
	default:
		err = fmt.Errorf("unknown command %q (want serve, migrate or openapi)", cmd)
	}
	if err != nil {
		slog.Error("core-api failed", "cmd", cmd, "err", err)
		os.Exit(1)
	}
}

func printOpenAPI() error {
	// The contract does not depend on config, so no database or token is needed here.
	_, api := httpapi.New(nil, "", httpapi.Services{})
	spec, err := api.OpenAPI().YAML()
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(spec)
	return err
}

func migrate() error {
	url, err := config.LoadDatabaseURL()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return db.Migrate(ctx, url)
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.MigrateOnStart {
		if err := db.Migrate(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	searcher, err := newSearch(ctx, pool)
	if err != nil {
		return err
	}
	handler, _ := httpapi.New(pool, cfg.InternalToken, httpapi.Services{
		Search: searcher, Products: searcher, Runs: &runs.Reader{DB: pool},
	})
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		// No WriteTimeout: SSE responses stay open. Per-route timeouts are set in handlers.
		IdleTimeout: 2 * time.Minute,
	}

	errc := make(chan error, 1)
	go func() {
		slog.Info("core-api listening", "addr", cfg.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		slog.Info("core-api shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// newSearch builds the search service: an insert-only River client for refresh
// jobs, the LISTEN hub for streams, and the query gate. Without an LLM, only
// queries already in the cache resolve.
func newSearch(ctx context.Context, pool *pgxpool.Pool) (*search.Service, error) {
	queue, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		return nil, err
	}
	gate := &guard.Gate{DB: pool}
	switch cfg, err := llm.ConfigFromEnv(); {
	case errors.Is(err, llm.ErrNotConfigured):
		slog.Info("LLM_API_KEY not set: only cached queries resolve")
	case err != nil:
		slog.Error("LLM config invalid: only cached queries resolve", "err", err)
	default:
		gate.Checker = &guard.Checker{Model: llm.NewOpenAI(cfg)}
	}
	hub := &search.Hub{DB: pool}
	go hub.Run(ctx)
	return &search.Service{DB: pool, Gate: gate, Quotas: &guard.Quotas{DB: pool}, Queue: queue, Hub: hub}, nil
}
