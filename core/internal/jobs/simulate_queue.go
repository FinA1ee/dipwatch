package jobs

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/FinA1ee/dipwatch/core/internal/alerts"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// Simulator queues simulated price drops for the admin API.
type Simulator struct {
	Queue interface {
		Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
	}
}

func (s Simulator) SimulateDrop(ctx context.Context, productID int64, percent int) error {
	_, err := s.Queue.Insert(ctx, SimulateDropArgs{ProductID: productID, Percent: percent}, nil)
	return err
}

// CheckTrackerArgs runs a product's alert rules on the stored offers, right
// after a member saves a tracker: a rule that already holds ("below ¥40,000"
// when the price is ¥36,000) alerts now rather than after the next refresh.
type CheckTrackerArgs struct {
	ProductID int64    `json:"product_id"`
	Sources   []string `json:"sources"`
}

func (CheckTrackerArgs) Kind() string { return "check_tracker" }

type CheckTrackerWorker struct {
	river.WorkerDefaults[CheckTrackerArgs]
	DB     *pgxpool.Pool
	Alerts *alerts.Checker
}

func (w *CheckTrackerWorker) Work(ctx context.Context, job *river.Job[CheckTrackerArgs]) error {
	q := store.New(w.DB)
	sources := job.Args.Sources
	if len(sources) == 0 {
		rows, err := q.ListEnabledSources(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			sources = append(sources, r.ID)
		}
	}
	for _, src := range sources {
		if _, err := w.Alerts.Check(ctx, q, job.Args.ProductID, src, alerts.Changes{}); err != nil {
			return err
		}
	}
	return nil
}

// CheckTracker queues the rule check for a product's tracker that was just saved.
func (s Simulator) CheckTracker(ctx context.Context, productID int64, sources []string) error {
	_, err := s.Queue.Insert(ctx, CheckTrackerArgs{ProductID: productID, Sources: sources}, nil)
	return err
}
