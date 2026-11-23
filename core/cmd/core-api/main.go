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

	"github.com/FinA1ee/dipwatch/core/internal/config"
	"github.com/FinA1ee/dipwatch/core/internal/db"
	httpapi "github.com/FinA1ee/dipwatch/core/internal/http"
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
	_, api := httpapi.New(nil, "")
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

	handler, _ := httpapi.New(pool, cfg.InternalToken)
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
