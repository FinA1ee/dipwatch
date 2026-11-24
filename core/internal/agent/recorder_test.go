package agent

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/llm"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// One run, end to end, recorded in a real database (spec C5).
func TestRunIsRecordedInAgentRuns(t *testing.T) {
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
	defer pool.Close()
	q := store.New(pool)

	before, err := q.SpendTodayMicros(ctx)
	if err != nil {
		t.Fatal(err)
	}

	model := &scriptedModel{turns: []llm.Turn{
		call("1", "browser_navigate", `{"url":"https://www.newegg.com/p/pl?d=rtx+5090"}`),
		call("2", "submit_offers", goodSubmission),
	}}
	a := &Agent{
		Model:      model,
		NewBrowser: func(context.Context) (Browser, error) { return &fakeBrowser{}, nil },
		Recorder:   DBRecorder{Queries: q},
	}
	// newegg is one of the seeded sources, so the foreign key holds.
	if _, err := a.Run(ctx, task); err != nil {
		t.Fatal(err)
	}

	var id int64
	if err := pool.QueryRow(ctx, `SELECT max(id) FROM app.agent_runs WHERE model = 'fake-model'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	row, err := q.GetAgentRun(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if row.Kind != "browse" || row.Outcome != OutcomeOK || row.Steps != 2 || row.TokensIn != 2000 ||
		row.TokensOut != 100 || row.CostCnyMicros != 2000*2+100*8 || row.SourceID == nil || *row.SourceID != "newegg" {
		t.Fatalf("recorded row: %+v", row)
	}
	var trace struct {
		Steps  []Step `json:"steps"`
		Offers int    `json:"offers"`
	}
	if err := json.Unmarshal(row.Trace, &trace); err != nil || len(trace.Steps) != 2 || trace.Offers != 2 {
		t.Fatalf("trace: %s (%v)", row.Trace, err)
	}

	after, err := q.SpendTodayMicros(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after-before != row.CostCnyMicros {
		t.Fatalf("spend today grew by %d, want %d", after-before, row.CostCnyMicros)
	}
}
