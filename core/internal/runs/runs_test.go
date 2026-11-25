package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

func setup(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
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
	return ctx, pool
}

func TestMetricsListAndTrace(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	sourceID := fmt.Sprintf("runs-test-%d", time.Now().UnixNano())
	if err := q.UpsertSource(ctx, store.UpsertSourceParams{ID: sourceID, Name: "Test", Region: "US", Currency: "USD", Strategies: []string{}}); err != nil {
		t.Fatal(err)
	}
	version := "browse-test-" + sourceID
	insert := func(outcome string, steps, offers int, cost int64) int64 {
		trace, _ := json.Marshal(map[string]any{
			"prompt_version": version, "offers": offers,
			"steps": []map[string]string{{"tool": "browser_navigate", "arguments": `{"url":"https://x.test"}`, "result": "page"}, {"note": "harness: budget 70% used"}},
		})
		id, err := q.InsertAgentRun(ctx, store.InsertAgentRunParams{
			Kind: "browse", SourceID: &sourceID, Model: "scripted", Steps: int32(steps), //nolint:gosec // test values
			TokensIn: 1000, TokensOut: 100, CostCnyMicros: cost, Outcome: outcome, Trace: trace,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	insert("ok", 4, 6, 50_000)
	insert("no_offers", 6, 0, 60_000)
	insert("token_limit", 15, 0, 70_000)
	last := insert("token_limit", 15, 0, 80_000)

	r := &Reader{DB: pool}
	m, err := r.Metrics(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	var day *SourceDay
	for i := range m.Sources {
		if m.Sources[i].SourceID == sourceID {
			day = &m.Sources[i]
		}
	}
	if day == nil {
		t.Fatalf("no metrics for %s", sourceID)
	}
	if day.Runs != 4 || day.SuccessRate != 0.5 || day.OffersPerRun != 1.5 || day.MedianSteps != 10.5 || day.TopFailure != "token_limit" || day.CostPerRun != 0.065 {
		t.Fatalf("metrics: %+v", *day)
	}

	list, err := r.List(ctx, Filter{SourceID: sourceID, Outcome: "token_limit", PromptVersion: version, Day: time.Now().UTC(), Limit: 10})
	if err != nil || len(list) != 2 || list[0].ID != last || list[0].PromptVersion != version {
		t.Fatalf("filtered list: %+v, %v", list, err)
	}
	if older, err := r.List(ctx, Filter{SourceID: sourceID, BeforeID: last, Limit: 10}); err != nil || len(older) != 3 {
		t.Fatalf("paging: %d runs, %v", len(older), err)
	}

	d, err := r.Get(ctx, last)
	if err != nil || len(d.Trace) != 2 || d.Trace[0].Tool != "browser_navigate" || d.Trace[1].Note == "" || d.Steps != 15 || d.Cost != 0.08 {
		t.Fatalf("detail: %+v, %v", d, err)
	}
}
