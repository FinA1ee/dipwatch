package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/FinA1ee/dipwatch/core/internal/alerts"
	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/limits"
	"github.com/FinA1ee/dipwatch/core/internal/money"
	"github.com/FinA1ee/dipwatch/core/internal/notify"
	"github.com/FinA1ee/dipwatch/core/internal/spend"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// NotifyArgs delivers one alert through its channel (T4).
type NotifyArgs struct {
	AlertID int64 `json:"alert_id"`
}

func (NotifyArgs) Kind() string { return "notify" }

func (NotifyArgs) InsertOpts() river.InsertOpts {
	// River retries with backoff: about 30 s, 1 min, 2 min, so a short Feishu
	// outage still lands within the 5 minutes of T4.
	return river.InsertOpts{MaxAttempts: 5}
}

// NotifyQueue is separate from refreshes, so an alert never waits behind slow
// agent runs and still goes out within the 5 minutes of T4.
const NotifyQueue = "notify"

// EnqueueNotify is the alerts.Checker enqueue function for jobs: it inserts a
// notify job with the River client of the job that found the alert.
func EnqueueNotify(ctx context.Context, alertID int64) error {
	client, err := river.ClientFromContextSafely[pgx.Tx](ctx)
	if err != nil {
		return err
	}
	_, err = client.Insert(ctx, NotifyArgs{AlertID: alertID}, &river.InsertOpts{Queue: NotifyQueue})
	return err
}

type NotifyWorker struct {
	river.WorkerDefaults[NotifyArgs]
	DB        *pgxpool.Pool
	Feishu    notify.Feishu
	Mailer    *notify.Mailer // nil when SMTP is not configured
	PublicURL string         // site address for links in messages
}

func (w *NotifyWorker) Timeout(*river.Job[NotifyArgs]) time.Duration { return 30 * time.Second }

func (w *NotifyWorker) Work(ctx context.Context, job *river.Job[NotifyArgs]) error {
	q := store.New(w.DB)
	a, err := q.GetAlertToSend(ctx, job.Args.AlertID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // the tracker was deleted meanwhile
	}
	if err != nil {
		return err
	}
	if a.Status == "sent" {
		return nil
	}
	subject, text := w.message(a)
	switch a.Channel {
	case alerts.ChannelFeishu:
		err = w.Feishu.Send(ctx, a.FeishuWebhook, subject+"\n"+text)
	case alerts.ChannelEmail:
		if w.Mailer == nil {
			err = errors.New("email alerts need SMTP_URL and MAIL_FROM")
		} else {
			err = w.Mailer.Send(ctx, a.Email, subject, text)
		}
	default:
		err = fmt.Errorf("unknown channel %q", a.Channel)
	}
	if err != nil {
		// Feishu answered but refused the message (bad token, bot removed): a retry
		// cannot succeed, so give up now. Network errors and timeouts are retried.
		var refused *notify.SendError
		permanent := errors.As(err, &refused) || errors.Is(err, notify.ErrWebhookHost)
		if permanent || job.Attempt >= job.MaxAttempts {
			_ = q.MarkAlert(ctx, store.MarkAlertParams{ID: a.ID, Status: "failed"})
		}
		slog.WarnContext(ctx, "notify: send failed", "alert", a.ID, "channel", a.Channel, "attempt", job.Attempt, "permanent", permanent, "err", err)
		if permanent {
			return river.JobCancel(err)
		}
		return err
	}
	slog.InfoContext(ctx, "notify: sent", "alert", a.ID, "channel", a.Channel, "product", a.ProductID)
	return q.MarkAlert(ctx, store.MarkAlertParams{ID: a.ID, Status: "sent"})
}

func (w *NotifyWorker) message(a store.GetAlertToSendRow) (subject, text string) {
	format := func(m alerts.Money) string { return money.Format(m.Minor, m.Currency) }
	title := a.TitleEn
	if title == "" {
		title = a.TitleOriginal
	}
	// Starts with "dipwatch" in lower case: Feishu bots often filter on that
	// keyword, and the match is case-sensitive.
	what := alerts.DescribeKey(a.Rule, format)
	if a.PreviousCnyMinor != nil && a.CurrentCnyMinor != nil && *a.PreviousCnyMinor > 0 {
		before, after := *a.PreviousCnyMinor, *a.CurrentCnyMinor
		direction := "up"
		if after < before {
			direction = "down"
		}
		what = fmt.Sprintf("price %s %.1f%%, %s to %s", direction,
			math.Abs(float64(after-before))*100/float64(before), money.Format(before, "CNY"), money.Format(after, "CNY"))
	}
	subject = fmt.Sprintf("dipwatch: %s, %s", a.CanonicalName, what)
	lines := []string{
		fmt.Sprintf("%s at %s: %s", money.Format(a.PriceMinor, a.Currency), a.SourceName, title),
		a.Url,
	}
	if w.PublicURL != "" {
		lines = append(lines, fmt.Sprintf("All offers and price history: %s/p/%d", strings.TrimRight(w.PublicURL, "/"), a.ProductID))
	}
	return subject, strings.Join(lines, "\n")
}

// forget deletes what the privacy page says is kept for 30 days: search
// counters by IP address, and expired sign-in sessions. A failure is logged:
// it must not stop refreshes, and the next run tries again.
func forget(ctx context.Context, q *store.Queries) {
	quota, err := q.DeleteOldQuota(ctx)
	if err != nil {
		slog.WarnContext(ctx, "schedule: delete old search counters", "err", err)
	}
	sessions, err := q.DeleteOldSessions(ctx)
	if err != nil {
		slog.WarnContext(ctx, "schedule: delete old sessions", "err", err)
	}
	if quota+sessions > 0 {
		slog.InfoContext(ctx, "schedule: deleted old records", "search_counters", quota, "sessions", sessions)
	}
}

// ScheduleArgs enqueues refreshes for tracked products that are due (T3).
type ScheduleArgs struct{}

func (ScheduleArgs) Kind() string { return "schedule_refreshes" }

// SchedulePeriodicJob runs the scheduler every 15 minutes.
func SchedulePeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(15*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return ScheduleArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}

type ScheduleWorker struct {
	river.WorkerDefaults[ScheduleArgs]
	DB     *pgxpool.Pool
	Spend  spend.Meter   // nil: no daily budget
	Limits *limits.Store // nil: the defaults (shortest re-check a member may choose)
}

func (w *ScheduleWorker) Work(ctx context.Context, job *river.Job[ScheduleArgs]) error {
	forget(ctx, store.New(w.DB))
	if spend.LevelOf(ctx, w.Spend) == spend.StoredOnly {
		slog.InfoContext(ctx, "schedule: paused, daily budget used up")
		return nil
	}
	l := limits.Defaults
	if w.Limits != nil {
		l = w.Limits.Get(ctx)
	}
	ids, err := store.New(w.DB).DueTrackedProducts(ctx, int32(max(l.MinCheckHours, 1)*3600)) //nolint:gosec // at most 24 hours
	if err != nil || len(ids) == 0 {
		return err
	}
	client, err := river.ClientFromContextSafely[pgx.Tx](ctx)
	if err != nil {
		return err
	}
	params := make([]river.InsertManyParams, len(ids))
	for i, p := range ids {
		args := RefreshProductArgs{ProductID: p.ID, Sources: p.Sources}
		if p.AllSources {
			args.Sources = nil // a tracker from before per-store tracking: every store
		}
		params[i] = river.InsertManyParams{Args: args, InsertOpts: &river.InsertOpts{Queue: job.Queue}}
	}
	if _, err := client.InsertMany(ctx, params); err != nil {
		return err
	}
	slog.InfoContext(ctx, "schedule: refreshes queued", "products", len(ids))
	return nil
}

// SimulateDropArgs lowers the offer a tracker of the product follows and runs
// the alert check, to test delivery end to end (M6.3). Admin only, through the API.
type SimulateDropArgs struct {
	ProductID int64 `json:"product_id"`
	Percent   int   `json:"percent"`
}

func (SimulateDropArgs) Kind() string { return "simulate_drop" }

type SimulateDropWorker struct {
	river.WorkerDefaults[SimulateDropArgs]
	DB     *pgxpool.Pool
	Alerts *alerts.Checker
}

func (w *SimulateDropWorker) Work(ctx context.Context, job *river.Job[SimulateDropArgs]) error {
	q := store.New(w.DB)
	offers, err := q.ListOffersByProduct(ctx, job.Args.ProductID)
	if err != nil {
		return err
	}
	rates, err := fx.Latest(ctx, q)
	if err != nil {
		rates = fx.Rates{}
	}
	// The offer a tracker would alert on: its pinned listing, else the cheapest
	// confirmed price of its size (any size without a tracker). Raw minor units
	// are not comparable across currencies, so compare in CNY.
	var pinned *int64
	variant := ""
	if watchers, err := q.ListProductWatchers(ctx, job.Args.ProductID); err == nil && len(watchers) > 0 {
		pinned, variant = watchers[0].OfferID, watchers[0].Variant
	}
	var target *store.AppOffer
	var best int64
	for i := range offers {
		o := &offers[i]
		if pinned != nil {
			if o.ID == *pinned {
				target = o
			}
			continue
		}
		if (o.IsProduct != nil && !*o.IsProduct) || o.PriceUnconfirmed || (variant != "" && o.Variant != variant) {
			continue
		}
		cny, err := rates.ToCNYMinor(o.PriceMinor, o.Currency)
		if err == nil && (target == nil || cny < best) {
			target, best = o, cny
		}
	}
	if target == nil {
		return fmt.Errorf("product %d has no offers to lower", job.Args.ProductID)
	}
	lowered := target.PriceMinor * int64(100-job.Args.Percent) / 100
	if err := q.SetOfferPrice(ctx, store.SetOfferPriceParams{ID: target.ID, PriceMinor: lowered}); err != nil {
		return err
	}
	inStock := true
	if _, err := q.InsertPricePointIfChanged(ctx, store.InsertPricePointIfChangedParams{
		OfferID: target.ID, PriceMinor: lowered, Currency: target.Currency, InStock: &inStock,
	}); err != nil {
		return err
	}
	n, err := w.Alerts.Check(ctx, q, job.Args.ProductID, target.SourceID, alerts.Changes{Changed: true})
	slog.InfoContext(ctx, "simulate: price lowered", "product", job.Args.ProductID, "offer", target.ID,
		"from", target.PriceMinor, "to", lowered, "alerts", n)
	return err
}
