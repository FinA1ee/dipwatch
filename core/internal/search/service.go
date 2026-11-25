package search

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/guard"
	"github.com/FinA1ee/dipwatch/core/internal/jobs"
	"github.com/FinA1ee/dipwatch/core/internal/money"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// FreshFor is how long a refresh's offers are served without starting another (S1).
const FreshFor = 6 * time.Hour

// Money is an amount in minor units (cents; yen for JPY) of an ISO 4217 currency.
type Money = money.Money

type Source struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

// Offer is one listing as shown in results (S5).
type Offer struct {
	ID            int64  `json:"id"`
	Title         string `json:"title" doc:"English title"`
	TitleOriginal string `json:"title_original" doc:"Title as the store shows it"`
	Source        Source `json:"source"`
	URL           string `json:"url" format:"uri"`
	Price         Money  `json:"price" doc:"Price in the store's currency"`
	Converted     *Money `json:"converted" doc:"Price in the requested currency; null when no exchange rate is stored"`
	ShippingMinor *int64 `json:"shipping_minor,omitempty" doc:"Shipping in the store's currency, when known"`
	Condition     string `json:"condition" enum:"new,used,refurbished,unknown"`
	Availability  string `json:"availability" enum:"in_stock,out_of_stock,unknown"`
	SellerType    string `json:"seller_type" enum:"official,third_party,individual,unknown"`
}

// Product is the resolved search target.
type Product struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Result answers one search.
type Result struct {
	Product Product `json:"product"`
	Status  string  `json:"status" enum:"fresh,refreshing" doc:"refreshing: open the events stream for new offers"`
	Offers  []Offer `json:"offers" doc:"Cheapest first, compared in CNY"`
}

// Inserter enqueues jobs. *river.Client satisfies it; tests use a fake.
type Inserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

type Service struct {
	DB     *pgxpool.Pool
	Gate   *guard.Gate
	Quotas *guard.Quotas // nil: no limits
	Queue  Inserter
	Hub    *Hub
	Now    func() time.Time // nil: time.Now
}

// Search resolves text to a product, returns its stored offers and, when they
// are older than FreshFor, starts a refresh (S1, S2). A search that needs the
// intent call or a refresh counts once against the viewer's daily quota (C1).
func (s *Service) Search(ctx context.Context, viewer guard.Viewer, text, currency string) (Result, error) {
	charge := &guard.Charger{Quotas: s.Quotas, Viewer: viewer}
	p, err := s.Gate.Resolve(ctx, text, charge)
	if err != nil {
		return Result{}, err
	}
	q := store.New(s.DB)
	product, err := q.GetProduct(ctx, p.ID)
	if err != nil {
		return Result{}, err
	}
	res := Result{Product: Product{ID: product.ID, Name: product.CanonicalName}, Status: "fresh"}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	fresh := product.LastRefreshedAt.Valid && now().Sub(product.LastRefreshedAt.Time) < FreshFor
	if !fresh || len(product.RefreshingSources) > 0 {
		res.Status = "refreshing"
	}
	if !fresh && len(product.RefreshingSources) == 0 {
		if err := charge.Charge(ctx); err != nil {
			return Result{}, err
		}
		// Unique per product while queued or running, so concurrent searches share it.
		if _, err := s.Queue.Insert(ctx, jobs.RefreshProductArgs{ProductID: product.ID}, nil); err != nil {
			return Result{}, err
		}
	}
	res.Offers, err = s.offers(ctx, q, product.ID, nil, currency)
	return res, err
}

// offers lists the product's offers (optionally one source's), converted to
// currency and ranked cheapest first by their CNY value.
func (s *Service) offers(ctx context.Context, q *store.Queries, productID int64, sourceID *string, currency string) ([]Offer, error) {
	rows, err := q.ListSearchOffers(ctx, store.ListSearchOffersParams{ProductID: productID, SourceID: sourceID})
	if err != nil {
		return nil, err
	}
	rates, err := fx.Latest(ctx, q)
	if err != nil {
		// Before the first rate fetch only CNY prices can be compared.
		slog.WarnContext(ctx, "search: no exchange rates", "err", err)
		rates = fx.Rates{}
	}

	type ranked struct {
		offer Offer
		cny   int64
		known bool
	}
	list := make([]ranked, 0, len(rows))
	for _, r := range rows {
		o := Offer{
			ID: r.ID, Title: r.TitleEn, TitleOriginal: r.TitleOriginal,
			Source: Source{ID: r.SourceID, Name: r.SourceName, Region: r.SourceRegion},
			URL:    r.Url, Price: Money{Minor: r.PriceMinor, Currency: r.Currency}, ShippingMinor: r.ShippingMinor,
			Condition: r.Condition, Availability: r.Availability, SellerType: r.SellerType,
		}
		if o.Title == "" {
			o.Title = r.TitleOriginal // not matched yet
		}
		item := ranked{offer: o}
		if cny, err := rates.ToCNYMinor(r.PriceMinor, r.Currency); err == nil {
			item.cny, item.known = cny, true
			if converted, err := rates.FromCNYMinor(cny, currency); err == nil {
				item.offer.Converted = &Money{Minor: converted, Currency: currency}
			}
		}
		list = append(list, item)
	}
	slices.SortStableFunc(list, func(a, b ranked) int {
		switch {
		case a.known != b.known: // offers without a rate go last
			if a.known {
				return -1
			}
			return 1
		case a.cny != b.cny:
			if a.cny < b.cny {
				return -1
			}
			return 1
		}
		return 0
	})
	out := make([]Offer, len(list))
	for i, item := range list {
		out[i] = item.offer
	}
	return out, nil
}
