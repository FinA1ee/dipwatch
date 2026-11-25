package jobs

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// FXRefreshArgs fetches and stores today's exchange rates.
type FXRefreshArgs struct{}

func (FXRefreshArgs) Kind() string { return "fx_refresh" }

type FXRefreshWorker struct {
	river.WorkerDefaults[FXRefreshArgs]
	DB     *pgxpool.Pool
	Client *http.Client
	URL    string
}

func (w *FXRefreshWorker) Work(ctx context.Context, _ *river.Job[FXRefreshArgs]) error {
	r, err := fx.Refresh(ctx, store.New(w.DB), w.Client, w.URL)
	if err != nil && r.Rates == nil {
		return err // nothing stored yet either: retry later
	}
	if err != nil {
		// The API is down but stored rates exist, which is what conversion will use.
		slog.WarnContext(ctx, "fx: fetch failed, keeping stored rates", "date", r.Date.Format(time.DateOnly), "err", err)
		return nil
	}
	slog.InfoContext(ctx, "fx: rates stored", "date", r.Date.Format(time.DateOnly), "currencies", len(r.Rates))
	return nil
}

// FXPeriodicJob runs the refresh at start and then every 6 hours. The API
// publishes once a working day; the extra runs only matter after an outage.
func FXPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(6*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return FXRefreshArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}
