// Package jobs holds River job definitions and their workers.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/FinA1ee/dipwatch/core/internal/sources"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

const offersPerSource = 50

// RefreshProductArgs asks the worker to fetch offers for one product from every
// enabled source. Only one pending job per product exists at a time, so many
// trackers of the same product share one refresh (design: Refresh policy and cost).
type RefreshProductArgs struct {
	ProductID int64 `json:"product_id"`
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

type RefreshProductWorker struct {
	river.WorkerDefaults[RefreshProductArgs]
	DB       *pgxpool.Pool
	Registry *sources.Registry
	Adapters Adapters
}

func (w *RefreshProductWorker) Timeout(*river.Job[RefreshProductArgs]) time.Duration {
	return 5 * time.Minute
}

func (w *RefreshProductWorker) Work(ctx context.Context, job *river.Job[RefreshProductArgs]) error {
	q := store.New(w.DB)
	product, err := q.GetProduct(ctx, job.Args.ProductID)
	if err != nil {
		return fmt.Errorf("load product %d: %w", job.Args.ProductID, err)
	}
	rows, err := q.ListEnabledSources(ctx)
	if err != nil {
		return fmt.Errorf("list sources: %w", err)
	}

	query := sources.Query{Product: product.CanonicalName, Limit: offersPerSource}
	for _, row := range rows {
		src, ok := w.source(row)
		if !ok {
			continue
		}
		// One source failing must not stop the others (spec S7), so errors are logged, not returned.
		res, err := w.Registry.Search(ctx, src, query)
		if err != nil {
			slog.WarnContext(ctx, "refresh: source failed", "product", product.ID, "source", src.ID, "err", err)
			continue
		}
		if err := w.save(ctx, product.ID, src.ID, res.Offers); err != nil {
			return fmt.Errorf("save offers from %s: %w", src.ID, err)
		}
		slog.InfoContext(ctx, "refresh: source done", "product", product.ID, "source", src.ID,
			"strategy", res.Strategy, "offers", len(res.Offers))
	}
	return q.MarkProductRefreshed(ctx, product.ID)
}

func (w *RefreshProductWorker) source(row store.AppSource) (sources.Source, bool) {
	byName := w.Adapters[row.ID]
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
func (w *RefreshProductWorker) save(ctx context.Context, productID int64, sourceID string, offers []sources.Offer) error {
	return pgx.BeginFunc(ctx, w.DB, func(tx pgx.Tx) error {
		q := store.New(tx)
		for _, o := range offers {
			id, err := q.UpsertOffer(ctx, store.UpsertOfferParams{
				ProductID:     productID,
				SourceID:      sourceID,
				ExternalID:    o.ExternalID,
				Url:           o.URL,
				TitleOriginal: o.Title,
				PriceMinor:    o.PriceMinor,
				Currency:      o.Currency,
				ShippingMinor: o.ShippingMinor,
				Condition:     orUnknown(o.Condition),
				Availability:  orUnknown(o.Availability),
				SellerType:    orUnknown(o.SellerType),
				Raw:           o.Raw,
			})
			if err != nil {
				return fmt.Errorf("upsert offer %s: %w", o.ExternalID, err)
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
