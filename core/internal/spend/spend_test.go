package spend

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/limits"
)

func TestLevels(t *testing.T) {
	for _, c := range []struct {
		today, budget float64
		want          Level
	}{
		{0, 5, Normal}, {3.99, 5, Normal}, {4, 5, TrackedOnly}, {4.99, 5, TrackedOnly},
		{5, 5, StoredOnly}, {7, 5, StoredOnly}, {0, 0, StoredOnly}, // a zero budget means no LLM spend at all
	} {
		if got := levelOf(c.today, c.budget); got != c.want {
			t.Errorf("levelOf(%v, %v) = %s, want %s", c.today, c.budget, got, c.want)
		}
	}
	if LevelOf(context.Background(), nil) != Normal {
		t.Error("nil meter should allow everything")
	}
}

func TestBreakerReadsTheChinaDayAndTheLiveBudget(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, url); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	l := &limits.Store{DB: pool}
	saved := l.Get(ctx)
	t.Cleanup(func() { _, _ = l.Save(context.Background(), saved) })

	// A day in the future has no runs, so spend is 0 whatever this database holds.
	future := time.Date(2099, 1, 1, 12, 0, 0, 0, China)
	b := &Breaker{DB: pool, Limits: l, Now: func() time.Time { return future }}
	next := saved
	next.DailyBudgetCNY = 5
	if _, err := l.Save(ctx, next); err != nil {
		t.Fatal(err)
	}
	st, err := b.Status(ctx)
	if err != nil || st.TodayCNY != 0 || st.BudgetCNY != 5 || b.Level(ctx) != Normal {
		t.Fatalf("status %+v, %v", st, err)
	}
	next.DailyBudgetCNY = 0
	if _, err := l.Save(ctx, next); err != nil {
		t.Fatal(err)
	}
	if b.Level(ctx) != StoredOnly {
		t.Fatal("a zero budget must stop LLM spend")
	}
}
