package db

import (
	"context"
	"fmt"
	neturl "net/url"
	"os"
	"testing"
	"time"
)

// Runs against a real Postgres. CI provides one; locally run `make db` first.
func TestMigrateCreatesAppSchemaAndIsIdempotent(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()

	for i := range 2 {
		if err := Migrate(ctx, url); err != nil {
			t.Fatalf("migrate run %d: %v", i+1, err)
		}
	}

	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var schemas int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.schemata WHERE schema_name = 'app'`).Scan(&schemas)
	if err != nil {
		t.Fatal(err)
	}
	if schemas != 1 {
		t.Fatalf("schema app: got %d, want 1", schemas)
	}

	var trgm int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_extension WHERE extname = 'pg_trgm'`).Scan(&trgm)
	if err != nil {
		t.Fatal(err)
	}
	if trgm != 1 {
		t.Fatalf("pg_trgm extension: got %d, want 1", trgm)
	}
}

// Two processes (or test packages) starting at once must not run migrations in parallel.
// Uses a throwaway database so the migrations really run from scratch.
func TestConcurrentMigrateOnFreshDatabase(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	admin, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	name := fmt.Sprintf("dipwatch_migrate_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})

	u, err := neturl.Parse(url)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	freshURL := u.String()

	const runs = 4
	errs := make(chan error, runs)
	for range runs {
		go func() { errs <- Migrate(ctx, freshURL) }()
	}
	for range runs {
		if err := <-errs; err != nil {
			t.Errorf("concurrent migrate: %v", err)
		}
	}
}
