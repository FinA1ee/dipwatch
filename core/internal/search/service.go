package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/guard"
	"github.com/FinA1ee/dipwatch/core/internal/jobs"
	"github.com/FinA1ee/dipwatch/core/internal/limits"
	"github.com/FinA1ee/dipwatch/core/internal/money"
	"github.com/FinA1ee/dipwatch/core/internal/spend"
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
	Product    Product `json:"product"`
	Status     string  `json:"status" enum:"fresh,refreshing" doc:"refreshing: open the events stream for new offers"`
	Paused     bool    `json:"paused,omitempty" doc:"true when stored offers are older than usual because today's budget is spent"`
	InviteOnly bool    `json:"invite_only,omitempty" doc:"true when stored offers were not refreshed because the viewer is not invited"`
	Offers     []Offer `json:"offers" doc:"Cheapest first, compared in CNY"`
}

// Inserter enqueues jobs. *river.Client satisfies it; tests use a fake.
type Inserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

type Service struct {
	DB     *pgxpool.Pool
	Gate   *guard.Gate
	Quotas *guard.Quotas // nil: no limits
	Limits *limits.Store // nil: the defaults
	Spend  spend.Meter   // nil: no daily budget
	Queue  Inserter
	Hub    *Hub
	Now    func() time.Time // nil: time.Now
}

// StoreError is a problem with the stores a search names; the API answers 400.
type StoreError string

func (e StoreError) Error() string { return string(e) }

// Store is one store a search can use.
type Store struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

// Stores lists the enabled stores and how many one search may use.
func (s *Service) Stores(ctx context.Context) ([]Store, int, error) {
	rows, err := store.New(s.DB).ListEnabledSources(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Store, len(rows))
	for i, r := range rows {
		out[i] = Store{ID: r.ID, Name: r.Name, Region: r.Region}
	}
	return out, s.limits(ctx).SourcesPerSearch, nil
}

func (s *Service) limits(ctx context.Context) limits.Limits {
	if s.Limits == nil {
		return limits.Defaults
	}
	return s.Limits.Get(ctx)
}

// checkStores keeps only enabled stores and enforces the per-search limit.
func (s *Service) checkStores(ctx context.Context, ids []string) ([]string, error) {
	enabled, max, err := s.Stores(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, StoreError("pick a store to search")
	}
	if len(ids) > max {
		return nil, StoreError(fmt.Sprintf("you can search up to %d store(s) at a time", max))
	}
	var out []string
	for _, id := range ids {
		if !slices.ContainsFunc(enabled, func(e Store) bool { return e.ID == id }) {
			return nil, StoreError(fmt.Sprintf("store %q is not available", id))
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

// Search resolves text to a product and returns its stored offers from the
// chosen stores. Stores whose offers are older than FreshFor are refreshed (S1,
// S2); only the chosen stores are searched, to bound spend (task 6.7). A search
// that needs the intent call or a refresh counts once against the daily quota (C1).
func (s *Service) Search(ctx context.Context, viewer guard.Viewer, text, currency string, stores []string) (Result, error) {
	stores, err := s.checkStores(ctx, stores)
	if err != nil {
		return Result{}, err
	}
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
	freshness, err := q.SourceFreshness(ctx, store.SourceFreshnessParams{ProductID: product.ID, SourceIds: stores})
	if err != nil {
		return Result{}, err
	}
	var stale []string
	for _, id := range stores {
		fresh := slices.ContainsFunc(freshness, func(f store.SourceFreshnessRow) bool {
			return f.SourceID == id && now().Sub(f.LastRefreshedAt.Time) < FreshFor
		})
		switch {
		case slices.Contains(product.RefreshingSources, id):
			res.Status = "refreshing" // already running: join it for free
		case !fresh:
			stale = append(stale, id)
		}
	}
	if len(stale) > 0 {
		// C4: past 80% of today's budget only tracked products are refreshed; at
		// 100% nothing is, and the stored offers are the answer.
		switch spend.LevelOf(ctx, s.Spend) {
		case spend.StoredOnly:
			stale, res.Paused = nil, true
		case spend.TrackedOnly:
			if tracked, err := q.ProductTracked(ctx, product.ID); err != nil || !tracked {
				stale, res.Paused = nil, true
			}
		}
	}
	if len(stale) > 0 {
		err := charge.Charge(ctx)
		if errors.Is(err, guard.ErrInviteOnly) {
			// Not invited: the stored offers are the answer, if there are any.
			res.Offers, err = s.offers(ctx, q, product.ID, stores, currency)
			if err == nil && len(res.Offers) == 0 {
				err = guard.ErrInviteOnly
			}
			res.InviteOnly = err == nil
			return res, err
		}
		if err != nil {
			return Result{}, err
		}
		// Unique while queued or running, so concurrent searches share it.
		if _, err := s.Queue.Insert(ctx, jobs.RefreshProductArgs{ProductID: product.ID, Sources: stale}, nil); err != nil {
			return Result{}, err
		}
		res.Status = "refreshing"
	}
	res.Offers, err = s.offers(ctx, q, product.ID, stores, currency)
	return res, err
}

// offers lists the product's offers (from the given stores; nil: all), converted to
// currency and ranked cheapest first by their CNY value.
func (s *Service) offers(ctx context.Context, q *store.Queries, productID int64, stores []string, currency string) ([]Offer, error) {
	if stores == nil {
		stores = []string{} // every store
	}
	rows, err := q.ListSearchOffers(ctx, store.ListSearchOffersParams{ProductID: productID, SourceIds: stores})
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
