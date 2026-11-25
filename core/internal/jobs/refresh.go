// Package jobs holds River job definitions and their workers.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/FinA1ee/dipwatch/core/internal/alerts"
	"github.com/FinA1ee/dipwatch/core/internal/matcher"
	"github.com/FinA1ee/dipwatch/core/internal/sources"
	"github.com/FinA1ee/dipwatch/core/internal/spend"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

const offersPerSource = 50

// RefreshProductArgs asks the worker to fetch offers for one product from every
// enabled source, as one RefreshSourceArgs job per source. Only one pending job per product exists at a time, so many
// trackers of the same product share one refresh (design: Refresh policy and cost).
type RefreshProductArgs struct {
	ProductID int64    `json:"product_id"`
	Sources   []string `json:"sources,omitempty"` // stores to search; empty: every enabled store
}

func (RefreshProductArgs) Kind() string { return "refresh_product" }

func (RefreshProductArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{
		ByArgs: true,
		// Unique only while waiting or running. River's default also counts completed
		// jobs, which would block every later refresh of the same product.
		ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRetryable,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
		},
	}}
}

// Adapters maps source id to strategy name to its implementation. A source in the
// database runs only the strategies that have an adapter here.
type Adapters map[string]map[string]sources.Strategy

// OffersChannel is the Postgres NOTIFY channel for finished sources. core-api
// listens on it to stream results (spec S2).
const OffersChannel = "dipwatch_offers"

// SourceDone is the NOTIFY payload sent when one source of a refresh finishes,
// whether it found offers or failed.
type SourceDone struct {
	ProductID int64  `json:"product_id"`
	SourceID  string `json:"source_id,omitempty"` // empty when the refresh had no sources
	OK        bool   `json:"ok"`
	Remaining int    `json:"remaining"` // sources of this refresh still running
}

// RefreshProductWorker fans a product refresh out to one job per source, so each
// source's results can be shown as soon as that source finishes.
type RefreshProductWorker struct {
	river.WorkerDefaults[RefreshProductArgs]
	DB       *pgxpool.Pool
	Adapters Adapters
	Spend    spend.Meter // nil: no daily budget
}

// allowed applies the spend breaker (C4) to queued and scheduled refreshes,
// which may run after the budget was reached.
func allowed(ctx context.Context, m spend.Meter, q *store.Queries, productID int64) bool {
	switch spend.LevelOf(ctx, m) {
	case spend.StoredOnly:
		return false
	case spend.TrackedOnly:
		tracked, err := q.ProductTracked(ctx, productID)
		return err == nil && tracked
	}
	return true
}

func (w *RefreshProductWorker) Work(ctx context.Context, job *river.Job[RefreshProductArgs]) error {
	q := store.New(w.DB)
	if !allowed(ctx, w.Spend, q, job.Args.ProductID) {
		slog.InfoContext(ctx, "refresh: skipped, daily budget", "product", job.Args.ProductID)
		return nil
	}
	rows, err := q.ListEnabledSources(ctx)
	if err != nil {
		return fmt.Errorf("list sources: %w", err)
	}
	var ids []string
	var args []river.InsertManyParams
	for _, row := range rows {
		if len(job.Args.Sources) > 0 && !slices.Contains(job.Args.Sources, row.ID) {
			continue
		}
		if _, ok := sourceFor(w.Adapters, row); ok {
			ids = append(ids, row.ID)
			args = append(args, river.InsertManyParams{
				Args: RefreshSourceArgs{ProductID: job.Args.ProductID, SourceID: row.ID},
				// Same queue as the parent, so a dedicated queue (tests) keeps its jobs.
				InsertOpts: &river.InsertOpts{Queue: job.Queue},
			})
		}
	}
	productID := job.Args.ProductID
	if err := q.StartRefresh(ctx, store.StartRefreshParams{ID: productID, SourceIds: ids}); err != nil {
		return err
	}
	if len(args) == 0 {
		if err := q.MarkProductRefreshed(ctx, productID); err != nil {
			return err
		}
		return notifySourceDone(ctx, q, SourceDone{ProductID: productID, OK: true})
	}
	client, err := river.ClientFromContextSafely[pgx.Tx](ctx)
	if err != nil {
		return err
	}
	_, err = client.InsertMany(ctx, args)
	return err
}

// RefreshSourceArgs fetches offers for one product from one source.
type RefreshSourceArgs struct {
	ProductID int64  `json:"product_id"`
	SourceID  string `json:"source_id"`
}

func (RefreshSourceArgs) Kind() string { return "refresh_source" }

func (RefreshSourceArgs) InsertOpts() river.InsertOpts {
	// One attempt: a failed source is reported as finished (S7) and retried by the
	// next refresh, not by River, so the stream never waits on a retry.
	return river.InsertOpts{MaxAttempts: 1, UniqueOpts: RefreshProductArgs{}.InsertOpts().UniqueOpts}
}

type RefreshSourceWorker struct {
	river.WorkerDefaults[RefreshSourceArgs]
	DB       *pgxpool.Pool
	Registry *sources.Registry
	Adapters Adapters
	// Matcher classifies the source's listings after they are saved. Optional:
	// without it, offers stay unclassified (is_product null).
	Matcher *matcher.Matcher
	// Alerts checks tracking rules after each source; optional.
	Alerts *alerts.Checker
	// Spend is the daily budget breaker (C4); optional.
	Spend spend.Meter
}

func (w *RefreshSourceWorker) Timeout(*river.Job[RefreshSourceArgs]) time.Duration {
	// Strategies run in order; an agent run alone may take 90 s.
	return 5 * time.Minute
}

func (w *RefreshSourceWorker) Work(ctx context.Context, job *river.Job[RefreshSourceArgs]) error {
	ok := w.refresh(ctx, job.Args)
	// Report the source as finished even when it failed, so the stream can end.
	q := store.New(w.DB)
	remaining, err := q.FinishSourceRefresh(ctx, store.FinishSourceRefreshParams{ID: job.Args.ProductID, SourceID: job.Args.SourceID})
	if err != nil {
		return err
	}
	// Any successful source makes the product fresh. When every source fails it
	// stays stale, so the next search tries again instead of serving nothing for 6 hours.
	if ok {
		if err := q.MarkProductRefreshed(ctx, job.Args.ProductID); err != nil {
			return err
		}
		if err := q.MarkSourceRefreshed(ctx, store.MarkSourceRefreshedParams{ProductID: job.Args.ProductID, SourceID: job.Args.SourceID}); err != nil {
			return err
		}
	}
	if remaining == 0 {
		// The whole refresh is done: adapt the schedule to how much changed (T3).
		if err := q.AdaptRefreshInterval(ctx, job.Args.ProductID); err != nil {
			return err
		}
	}
	return notifySourceDone(ctx, q, SourceDone{ProductID: job.Args.ProductID, SourceID: job.Args.SourceID, OK: ok, Remaining: int(remaining)})
}

// refresh searches one source, saves and matches its offers, and reports whether
// it succeeded. Failures are logged, not returned: one source failing must not
// affect the others (spec S7).
func (w *RefreshSourceWorker) refresh(ctx context.Context, args RefreshSourceArgs) bool {
	q := store.New(w.DB)
	if !allowed(ctx, w.Spend, q, args.ProductID) {
		slog.InfoContext(ctx, "refresh: source skipped, daily budget", "product", args.ProductID, "source", args.SourceID)
		return false
	}
	product, err := q.GetProduct(ctx, args.ProductID)
	if err != nil {
		slog.ErrorContext(ctx, "refresh: load product", "product", args.ProductID, "err", err)
		return false
	}
	rows, err := q.ListEnabledSources(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "refresh: list sources", "err", err)
		return false
	}
	var src sources.Source
	found := false
	for _, row := range rows {
		if row.ID == args.SourceID {
			src, found = sourceFor(w.Adapters, row)
		}
	}
	if !found {
		slog.WarnContext(ctx, "refresh: source disabled or without adapter", "source", args.SourceID)
		return false
	}

	res, err := w.Registry.Search(ctx, src, sources.Query{Product: product.CanonicalName, Limit: offersPerSource})
	if err != nil {
		slog.WarnContext(ctx, "refresh: source failed", "product", product.ID, "source", src.ID, "err", err)
		return false
	}
	changes, err := save(ctx, w.DB, product.ID, src.ID, res.Offers)
	if err != nil {
		slog.ErrorContext(ctx, "refresh: save offers", "product", product.ID, "source", src.ID, "err", err)
		return false
	}
	if changes.Changed {
		if err := q.MarkRefreshChanged(ctx, product.ID); err != nil {
			slog.WarnContext(ctx, "refresh: mark changed", "product", product.ID, "err", err)
		}
	}
	slog.InfoContext(ctx, "refresh: source done", "product", product.ID, "source", src.ID,
		"strategy", res.Strategy, "offers", len(res.Offers))
	if w.Matcher != nil {
		if err := w.match(ctx, product, src.ID); err != nil {
			// Unclassified offers are still shown, and matched on the next refresh.
			slog.WarnContext(ctx, "refresh: matching failed", "product", product.ID, "source", src.ID, "err", err)
		}
	}
	// After matching, so accessories never trigger alerts.
	if w.Alerts != nil {
		if n, err := w.Alerts.Check(ctx, q, product.ID, src.ID, changes); err != nil {
			slog.ErrorContext(ctx, "refresh: alert check failed", "product", product.ID, "source", src.ID, "err", err)
		} else if n > 0 {
			slog.InfoContext(ctx, "refresh: alerts queued", "product", product.ID, "source", src.ID, "alerts", n)
		}
	}
	return true
}

func notifySourceDone(ctx context.Context, q *store.Queries, done SourceDone) error {
	payload, err := json.Marshal(done)
	if err != nil {
		return err
	}
	return q.NotifyOffers(ctx, string(payload))
}

func sourceFor(adapters Adapters, row store.AppSource) (sources.Source, bool) {
	byName := adapters[row.ID]
	src := sources.Source{ID: row.ID}
	for _, name := range row.Strategies {
		if s, ok := byName[name]; ok {
			src.Strategies = append(src.Strategies, s)
		}
	}
	return src, len(src.Strategies) > 0
}

// save upserts every offer and records a price point where the price or stock changed,
// all in one transaction per source.
func save(ctx context.Context, db *pgxpool.Pool, productID int64, sourceID string, offers []sources.Offer) (alerts.Changes, error) {
	ch := alerts.Changes{BackInStock: map[int64]bool{}}
	started := time.Now()
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		q := store.New(tx)
		seen, err := q.SourceSeenBefore(ctx, store.SourceSeenBeforeParams{
			ProductID: productID, SourceID: sourceID, Before: pgtype.Timestamptz{Time: started, Valid: true},
		})
		if err != nil {
			return err
		}
		ch.FirstFromSource = !seen && len(offers) > 0
		for _, o := range offers {
			id, err := q.UpsertOffer(ctx, store.UpsertOfferParams{
				ProductID:        productID,
				SourceID:         sourceID,
				ExternalID:       o.ExternalID,
				Url:              o.URL,
				TitleOriginal:    o.Title,
				PriceMinor:       o.PriceMinor,
				Currency:         o.Currency,
				ShippingMinor:    o.ShippingMinor,
				Condition:        orUnknown(o.Condition),
				Availability:     orUnknown(o.Availability),
				SellerType:       orUnknown(o.SellerType),
				Raw:              o.Raw,
				PriceUnconfirmed: o.PriceUnconfirmed,
			})
			if err != nil {
				return fmt.Errorf("upsert offer %s: %w", o.ExternalID, err)
			}
			prev, err := q.LatestPricePoint(ctx, id)
			switch {
			case errors.Is(err, pgx.ErrNoRows): // a new listing
			case err != nil:
				return err
			default:
				now := inStock(o.Availability)
				if prev.InStock != nil && !*prev.InStock && now != nil && *now {
					ch.BackInStock[id] = true
				}
				if prev.PriceMinor != o.PriceMinor || prev.Currency != o.Currency || !sameStock(prev.InStock, now) {
					ch.Changed = true
				}
			}
			if _, err := q.InsertPricePointIfChanged(ctx, store.InsertPricePointIfChangedParams{
				OfferID:    id,
				PriceMinor: o.PriceMinor,
				Currency:   o.Currency,
				InStock:    inStock(o.Availability),
			}); err != nil {
				return fmt.Errorf("record price of %s: %w", o.ExternalID, err)
			}
		}
		return nil
	})
	return ch, err
}

func sameStock(a, b *bool) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func inStock(availability string) *bool {
	var b bool
	switch availability {
	case "in_stock":
		b = true
	case "out_of_stock":
		b = false
	default:
		return nil
	}
	return &b
}

// match classifies one source's offers for the product. Decisions are cached per
// product and title, so the LLM only sees titles it has never seen.
func (w *RefreshSourceWorker) match(ctx context.Context, product store.AppProduct, sourceID string) error {
	q := store.New(w.DB)
	offers, err := q.ListOffersByProduct(ctx, product.ID)
	if err != nil {
		return err
	}
	// Every refresh re-applies decisions to all of the source's offers: the upsert
	// resets the condition and may change the title, and cached titles cost no LLM call.
	var pending []store.AppOffer
	for _, o := range offers {
		if o.SourceID == sourceID {
			pending = append(pending, o)
		}
	}
	if len(pending) == 0 {
		return nil
	}

	hashes := make([]string, len(pending))
	for i, o := range pending {
		hashes[i] = matcher.TitleHash(o.TitleOriginal)
	}
	cached, err := q.GetMatchDecisions(ctx, store.GetMatchDecisionsParams{ProductID: product.ID, TitleHashes: hashes})
	if err != nil {
		return err
	}
	known := map[string]matcher.Decision{}
	for _, c := range cached {
		known[c.TitleHash] = matcher.Decision{IsProduct: c.IsProduct, TitleEn: c.TitleEn, Condition: c.Condition, Variant: c.Variant}
	}

	var titles []string
	var newHashes []string
	for i, o := range pending {
		if _, ok := known[hashes[i]]; !ok && !contains(newHashes, hashes[i]) {
			titles = append(titles, o.TitleOriginal)
			newHashes = append(newHashes, hashes[i])
		}
	}
	if len(titles) > 0 {
		start := time.Now()
		decisions, usage, err := w.Matcher.Decide(ctx, product.CanonicalName, titles)
		w.recordMatch(ctx, product.ID, usage, time.Since(start), err)
		if err != nil {
			return err
		}
		for i, d := range decisions {
			known[newHashes[i]] = d
			if err := q.SaveMatchDecision(ctx, store.SaveMatchDecisionParams{
				ProductID: product.ID, TitleHash: newHashes[i], IsProduct: d.IsProduct, TitleEn: d.TitleEn, Condition: d.Condition, Variant: d.Variant,
			}); err != nil {
				return err
			}
		}
	}

	for i, o := range pending {
		d := known[hashes[i]]
		condition := d.Condition
		if condition == "unknown" {
			condition = o.Condition // keep what the source said when the title says nothing
		}
		isProduct := d.IsProduct
		if err := q.SetOfferMatch(ctx, store.SetOfferMatchParams{ID: o.ID, IsProduct: &isProduct, TitleEn: d.TitleEn, Condition: condition, Variant: d.Variant}); err != nil {
			return err
		}
	}
	slog.InfoContext(ctx, "refresh: matched", "product", product.ID, "offers", len(pending), "llm_titles", len(titles))
	return nil
}

func (w *RefreshSourceWorker) recordMatch(ctx context.Context, productID int64, u matcher.Usage, took time.Duration, err error) {
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	trace, _ := json.Marshal(map[string]any{"prompt_version": matcher.PromptVersion, "calls": u.Calls})
	model := "unknown"
	if w.Matcher.Model != nil {
		model = w.Matcher.Model.Name()
	}
	if _, rerr := store.New(w.DB).InsertAgentRun(ctx, store.InsertAgentRunParams{
		Kind: "match", ProductID: &productID, Model: model, Steps: int32(u.Calls), //nolint:gosec // small
		TokensIn: int32(min(u.TokensIn, 1<<31-1)), TokensOut: int32(min(u.TokensOut, 1<<31-1)), //nolint:gosec // bounded
		CostCnyMicros: u.CostMicros, DurationMs: int32(min(took.Milliseconds(), 1<<31-1)), //nolint:gosec // bounded
		Outcome: outcome, Trace: trace,
	}); rerr != nil {
		slog.WarnContext(ctx, "refresh: recording match run failed", "err", rerr)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
