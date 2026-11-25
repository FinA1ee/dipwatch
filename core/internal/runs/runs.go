// Package runs reads app.agent_runs for the admin pages: per-source metrics
// (spec H3) and run inspection with the step-by-step trace (H4).
package runs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/live"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// MaxDays bounds the metrics window.
const MaxDays = 30

type SourceDay struct {
	SourceID     string  `json:"source_id"`
	Day          string  `json:"day" format:"date"`
	Runs         int     `json:"runs"`
	SuccessRate  float64 `json:"success_rate" doc:"Share of runs that answered: offers, or a confirmed 'no listings'"`
	OffersPerRun float64 `json:"offers_per_run"`
	MedianSteps  float64 `json:"median_steps"`
	TokensPerRun float64 `json:"tokens_per_run"`
	CostPerRun   float64 `json:"cost_per_run_cny"`
	TopFailure   string  `json:"top_failure" doc:"Most common failed outcome; empty when none failed"`
}

type SpendDay struct {
	Kind  string  `json:"kind" enum:"browse,match,intent"`
	Day   string  `json:"day" format:"date"`
	Calls int     `json:"calls"`
	Cost  float64 `json:"cost_cny"`
}

type Metrics struct {
	Sources []SourceDay `json:"sources"`
	Spend   []SpendDay  `json:"spend"`
}

// Summary is one run in the list.
type Summary struct {
	ID            int64     `json:"id"`
	Kind          string    `json:"kind" enum:"browse,match,intent"`
	SourceID      string    `json:"source_id,omitempty"`
	ProductID     int64     `json:"product_id,omitempty"`
	Model         string    `json:"model"`
	PromptVersion string    `json:"prompt_version"`
	Outcome       string    `json:"outcome"`
	Offers        int       `json:"offers"`
	Steps         int       `json:"steps"`
	TokensIn      int       `json:"tokens_in"`
	TokensOut     int       `json:"tokens_out"`
	Cost          float64   `json:"cost_cny"`
	DurationMs    int       `json:"duration_ms"`
	CreatedAt     time.Time `json:"created_at"`
}

// Step is one entry of a browsing run's trace, the same shape the live log streams.
type Step = live.Step

// Detail is a run with its trace. The trace is not called "steps": that name is
// the step count in the embedded Summary.
type Detail struct {
	Summary
	Trace []Step `json:"trace"`
	Error string `json:"error,omitempty"`
}

// Filter narrows the run list; empty fields do not filter.
type Filter struct {
	Kind, SourceID, Outcome, PromptVersion string
	Day                                    time.Time // zero: any day
	BeforeID                               int64     // paging: runs older than this id
	Limit                                  int
}

type Reader struct {
	DB *pgxpool.Pool
}

func (r *Reader) Metrics(ctx context.Context, days int) (Metrics, error) {
	days = min(max(days, 1), MaxDays)
	since := pgtype.Timestamptz{Time: time.Now().UTC().AddDate(0, 0, -days+1).Truncate(24 * time.Hour), Valid: true}
	q := store.New(r.DB)
	rows, err := q.SourceRunMetrics(ctx, since)
	if err != nil {
		return Metrics{}, err
	}
	m := Metrics{Sources: make([]SourceDay, len(rows)), Spend: []SpendDay{}}
	for i, row := range rows {
		m.Sources[i] = SourceDay{
			SourceID: row.SourceID, Day: row.Day.Time.Format(time.DateOnly), Runs: int(row.Runs),
			SuccessRate:  float64(row.Answered) / float64(max(row.Runs, 1)),
			OffersPerRun: row.OffersPerRun, MedianSteps: row.MedianSteps, TokensPerRun: row.TokensPerRun,
			CostPerRun: row.CostMicrosPerRun / 1e6, TopFailure: row.TopFailure,
		}
	}
	spend, err := q.SpendByKind(ctx, since)
	if err != nil {
		return Metrics{}, err
	}
	for _, row := range spend {
		m.Spend = append(m.Spend, SpendDay{Kind: row.Kind, Day: row.Day.Time.Format(time.DateOnly), Calls: int(row.Calls), Cost: float64(row.CostMicros) / 1e6})
	}
	return m, nil
}

func (r *Reader) List(ctx context.Context, f Filter) ([]Summary, error) {
	params := store.ListAgentRunsParams{
		Kind: orNil(f.Kind), SourceID: orNil(f.SourceID), Outcome: orNil(f.Outcome), PromptVersion: orNil(f.PromptVersion),
		RowLimit: int32(min(max(f.Limit, 1), 200)), //nolint:gosec // bounded
	}
	if !f.Day.IsZero() {
		params.Day = pgtype.Date{Time: f.Day, Valid: true}
	}
	if f.BeforeID > 0 {
		params.BeforeID = &f.BeforeID
	}
	rows, err := store.New(r.DB).ListAgentRuns(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]Summary, len(rows))
	for i, row := range rows {
		out[i] = Summary{
			ID: row.ID, Kind: row.Kind, SourceID: deref(row.SourceID), ProductID: deref(row.ProductID), Model: row.Model,
			PromptVersion: row.PromptVersion, Outcome: row.Outcome, Offers: int(row.Offers), Steps: int(row.Steps),
			TokensIn: int(row.TokensIn), TokensOut: int(row.TokensOut), Cost: float64(row.CostCnyMicros) / 1e6,
			DurationMs: int(row.DurationMs), CreatedAt: row.CreatedAt.Time,
		}
	}
	return out, nil
}

// Get returns one run; pgx.ErrNoRows when it does not exist.
func (r *Reader) Get(ctx context.Context, id int64) (Detail, error) {
	row, err := store.New(r.DB).GetAgentRun(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	var trace struct {
		PromptVersion string `json:"prompt_version"`
		Steps         []Step `json:"steps"`
		Offers        int    `json:"offers"`
		Error         string `json:"error"`
	}
	if len(row.Trace) > 0 {
		if err := json.Unmarshal(row.Trace, &trace); err != nil {
			return Detail{}, err
		}
	}
	if trace.Steps == nil {
		trace.Steps = []Step{} // matcher and intent runs are single calls without steps
	}
	return Detail{
		Summary: Summary{
			ID: row.ID, Kind: row.Kind, SourceID: deref(row.SourceID), ProductID: deref(row.ProductID), Model: row.Model,
			PromptVersion: trace.PromptVersion, Outcome: row.Outcome, Offers: trace.Offers, Steps: int(row.Steps),
			TokensIn: int(row.TokensIn), TokensOut: int(row.TokensOut), Cost: float64(row.CostCnyMicros) / 1e6,
			DurationMs: int(row.DurationMs), CreatedAt: row.CreatedAt.Time,
		},
		Trace: trace.Steps,
		Error: trace.Error,
	}, nil
}

func orNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
