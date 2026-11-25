package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/guard"
	"github.com/FinA1ee/dipwatch/core/internal/jobs"
	"github.com/FinA1ee/dipwatch/core/internal/limits"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

type fakeQueue struct{ inserted []river.JobArgs }

func (f *fakeQueue) Insert(_ context.Context, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	f.inserted = append(f.inserted, args)
	return &rivertype.JobInsertResult{}, nil
}

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

// fixture creates a product reachable by the query "<suffix> card", one source
// with three offers in two currencies, and exchange rates.
func fixture(t *testing.T, ctx context.Context, q *store.Queries) (product store.AppProduct, sourceID, query string) {
	t.Helper()
	suffix := fmt.Sprint(time.Now().UnixNano())
	product, err := q.UpsertProduct(ctx, store.UpsertProductParams{CanonicalName: "Search Test " + suffix, Category: "gpu"})
	if err != nil {
		t.Fatal(err)
	}
	query = suffix + " card"
	if err := q.SaveQuery(ctx, store.SaveQueryParams{NormalizedText: query, ProductID: &product.ID}); err != nil {
		t.Fatal(err)
	}
	sourceID = "search-test-" + suffix
	// Enabled, so searches may name it; without strategies no worker ever runs it.
	if err := q.UpsertSource(ctx, store.UpsertSourceParams{ID: sourceID, Name: "Test store", Region: "US", Currency: "USD", Strategies: []string{}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = q.UpsertSource(context.Background(), store.UpsertSourceParams{ID: sourceID, Name: "Test store", Region: "US", Currency: "USD", Strategies: []string{}})
	})
	// Dated long ago, so they never replace real rates in a shared dev database.
	// The order below holds for any USD rate between 0.1 and 0.2.
	if err := fx.Save(ctx, q, fx.Rates{Date: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), Rates: map[string]float64{"USD": 0.14}}); err != nil {
		t.Fatal(err)
	}
	notProduct := false
	for _, o := range []struct {
		id       string
		minor    int64
		currency string
		match    *bool
	}{
		{"cny-100", 10000, "CNY", nil}, // ¥100
		{"usd-10", 1000, "USD", nil},   // about ¥70
		{"cny-50", 5000, "CNY", nil},   // ¥50
		{"cable", 100, "CNY", &notProduct},
	} {
		id, err := q.UpsertOffer(ctx, store.UpsertOfferParams{
			ProductID: product.ID, SourceID: sourceID, ExternalID: o.id, Url: "https://example.test/" + o.id,
			TitleOriginal: o.id, PriceMinor: o.minor, Currency: o.currency,
			Condition: "new", Availability: "in_stock", SellerType: "unknown",
		})
		if err != nil {
			t.Fatal(err)
		}
		if o.match != nil {
			if err := q.SetOfferMatch(ctx, store.SetOfferMatchParams{ID: id, IsProduct: o.match, TitleEn: o.id, Condition: "new"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	return product, sourceID, query
}

func ids(offers []Offer) []string {
	out := make([]string, len(offers))
	for i, o := range offers {
		out[i] = o.TitleOriginal
	}
	return out
}

func TestSearchRanksInCNYAndStartsRefresh(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	product, sourceID, query := fixture(t, ctx, q)
	queue := &fakeQueue{}
	s := &Service{DB: pool, Gate: &guard.Gate{DB: pool}, Queue: queue}

	res, err := s.Search(ctx, guard.Viewer{}, "  "+query+" ", "USD", []string{sourceID})
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(ids(res.Offers)); got != "[cny-50 usd-10 cny-100]" {
		t.Fatalf("ranking: %s (the rejected cable must be left out)", got)
	}
	if usd := res.Offers[1].Converted; usd == nil || usd.Currency != "USD" || usd.Minor != 1000 {
		t.Fatalf("USD offer converted to USD: %+v", usd)
	}
	if res.Status != "refreshing" || len(queue.inserted) != 1 {
		t.Fatalf("never refreshed: status %s, %d jobs", res.Status, len(queue.inserted))
	}

	// Fresh: served from the database, no new job (S1).
	if err := q.MarkSourceRefreshed(ctx, store.MarkSourceRefreshedParams{ProductID: product.ID, SourceID: sourceID}); err != nil {
		t.Fatal(err)
	}
	res, err = s.Search(ctx, guard.Viewer{}, query, "CNY", []string{sourceID})
	if err != nil || res.Status != "fresh" || len(queue.inserted) != 1 {
		t.Fatalf("fresh search: status %s, %d jobs, %v", res.Status, len(queue.inserted), err)
	}
}

func TestStreamSendsOffersUntilSourcesFinish(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	product, sourceID, _ := fixture(t, ctx, q)
	if err := q.StartRefresh(ctx, store.StartRefreshParams{ID: product.ID, SourceIds: []string{sourceID}}); err != nil {
		t.Fatal(err)
	}
	hubCtx, stopHub := context.WithCancel(ctx)
	defer stopHub()
	hub := &Hub{DB: pool}
	go hub.Run(hubCtx)
	s := &Service{DB: pool, Hub: hub}

	events := make(chan any, 8)
	errc := make(chan error, 1)
	go func() {
		errc <- s.Stream(ctx, product.ID, "CNY", []string{sourceID}, func(e any) error { events <- e; return nil })
	}()

	next := func() any {
		t.Helper()
		select {
		case e := <-events:
			return e
		case <-time.After(5 * time.Second):
			t.Fatal("no event within 5s")
			return nil
		}
	}
	if e, ok := next().(OffersEvent); !ok || len(e.Pending) != 1 || len(e.Offers) != 3 {
		t.Fatalf("first event: %+v", e)
	}

	// The worker finishes the source.
	time.Sleep(200 * time.Millisecond) // let the hub's LISTEN start
	if _, err := q.FinishSourceRefresh(ctx, store.FinishSourceRefreshParams{ID: product.ID, SourceID: sourceID}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(jobs.SourceDone{ProductID: product.ID, SourceID: sourceID, OK: true})
	if err := q.NotifyOffers(ctx, string(payload)); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if e, ok := next().(OffersEvent); !ok || len(e.Pending) != 0 {
		t.Fatalf("second event: %+v", e)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("update took %s: notification not used", took)
	}
	if e, ok := next().(DoneEvent); !ok || e.TimedOut {
		t.Fatalf("last event: %+v", e)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
}

func TestQuotaCountsOnlyCostlySearches(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	product, sourceID, query := fixture(t, ctx, q)
	s := &Service{DB: pool, Gate: &guard.Gate{DB: pool}, Quotas: &guard.Quotas{DB: pool}, Queue: &fakeQueue{}}
	unique := fmt.Sprint(time.Now().UnixNano())

	// The product is stale and the fake queue never starts the refresh, so every
	// search is costly.
	visitor := guard.Viewer{IP: "198.51.100." + unique}
	for i := range limits.Defaults.VisitorDailySearches {
		if _, err := s.Search(ctx, visitor, query, "CNY", []string{sourceID}); err != nil {
			t.Fatalf("visitor search %d: %v", i+1, err)
		}
	}
	if _, err := s.Search(ctx, visitor, query, "CNY", []string{sourceID}); !errors.Is(err, guard.ErrQuotaExceeded) {
		t.Fatalf("visitor search %d: %v, want quota exceeded", limits.Defaults.VisitorDailySearches+1, err)
	}

	member := guard.Viewer{UserID: "quota-test-" + unique, IP: visitor.IP} // same IP, own quota
	for i := range limits.Defaults.MemberDailySearches {
		if _, err := s.Search(ctx, member, query, "CNY", []string{sourceID}); err != nil {
			t.Fatalf("member search %d: %v", i+1, err)
		}
	}
	if _, err := s.Search(ctx, member, query, "CNY", []string{sourceID}); !errors.Is(err, guard.ErrQuotaExceeded) {
		t.Fatalf("member search %d: %v, want quota exceeded", limits.Defaults.MemberDailySearches+1, err)
	}

	// A fresh product is answered from the database: free even over the limit.
	if err := q.MarkSourceRefreshed(ctx, store.MarkSourceRefreshedParams{ProductID: product.ID, SourceID: sourceID}); err != nil {
		t.Fatal(err)
	}
	if res, err := s.Search(ctx, visitor, query, "CNY", []string{sourceID}); err != nil || res.Status != "fresh" {
		t.Fatalf("cached search over the limit: %+v, %v", res, err)
	}
}

func TestSearchNamesItsStoresWithinTheLimit(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	_, sourceID, query := fixture(t, ctx, q)
	_, other, _ := fixture(t, ctx, q)
	l := &limits.Store{DB: pool}
	saved := l.Get(ctx)
	t.Cleanup(func() { _, _ = l.Save(context.Background(), saved) })
	s := &Service{DB: pool, Gate: &guard.Gate{DB: pool}, Limits: l, Queue: &fakeQueue{}}

	var storeErr StoreError
	if _, err := s.Search(ctx, guard.Viewer{}, query, "CNY", nil); !errors.As(err, &storeErr) {
		t.Fatalf("no store: %v", err)
	}
	if _, err := s.Search(ctx, guard.Viewer{}, query, "CNY", []string{"no-such-store"}); !errors.As(err, &storeErr) {
		t.Fatalf("unknown store: %v", err)
	}
	if _, err := l.Save(ctx, limits.Limits{SourcesPerSearch: 1, TrackedPerMember: 1, VisitorDailySearches: 10, MemberDailySearches: 30}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search(ctx, guard.Viewer{}, query, "CNY", []string{sourceID, other}); !errors.As(err, &storeErr) {
		t.Fatalf("two stores with a limit of 1: %v", err)
	}
	// The owner raises the limit: it applies to the next search.
	if _, err := l.Save(ctx, limits.Limits{SourcesPerSearch: 2, TrackedPerMember: 1, VisitorDailySearches: 10, MemberDailySearches: 30}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Search(ctx, guard.Viewer{}, query, "CNY", []string{sourceID, other})
	if err != nil {
		t.Fatalf("two stores with a limit of 2: %v", err)
	}
	for _, o := range res.Offers {
		if o.Source.ID != sourceID && o.Source.ID != other {
			t.Fatalf("offer from a store not asked for: %s", o.Source.ID)
		}
	}
	if q := s.Queue.(*fakeQueue); len(q.inserted) != 1 || len(q.inserted[0].(jobs.RefreshProductArgs).Sources) != 2 {
		t.Fatalf("refresh: %+v", q.inserted)
	}
}
