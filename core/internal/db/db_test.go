package db

import (
	"context"
	"os"
	"testing"
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
