package jobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/sources"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

type fakeStrategy struct{ offers []sources.Offer }

func (f *fakeStrategy) Name() string { return "api" }

func (f *fakeStrategy) Search(context.Context, sources.Query) ([]sources.Offer, error) {
	return f.offers, nil
}

type failingStrategy struct{}

func (failingStrategy) Name() string { return "api" }

func (failingStrategy) Search(context.Context, sources.Query) ([]sources.Offer, error) {
	return nil, errors.New("blocked")
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

// runJob starts a River client with the given adapters, enqueues one refresh job and
// waits for it to finish.
func runJob(t *testing.T, ctx context.Context, pool *pgxpool.Pool, adapters Adapters, productID int64) {
	t.Helper()
	workers := river.NewWorkers()
	river.AddWorker(workers, &RefreshProductWorker{DB: pool, Registry: sources.NewRegistry(), Adapters: adapters})
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}},
		Workers: workers,
	})
	if err != nil {
		t.Fatal(err)
	}
	done, cancel := client.Subscribe(river.EventKindJobCompleted, river.EventKindJobFailed)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Stop(ctx) }()

	res, err := client.Insert(ctx, RefreshProductArgs{ProductID: productID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	timeout := time.After(15 * time.Second)
	for {
		select {
		case ev := <-done:
			if ev.Job.ID != res.Job.ID {
				continue // a job left over from another test run
			}
			if ev.Kind != river.EventKindJobCompleted {
				t.Fatalf("job %s: %v", ev.Kind, ev.Job.Errors)
			}
			return
		case <-timeout:
			t.Fatal("refresh job did not finish within 15s")
		}
	}
}

func newProductAndSource(t *testing.T, ctx context.Context, q *store.Queries) (store.AppProduct, string) {
	t.Helper()
	suffix := fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())
	product, err := q.UpsertProduct(ctx, store.UpsertProductParams{CanonicalName: "Test GPU " + suffix, Category: "gpu"})
	if err != nil {
		t.Fatal(err)
	}
	sourceID := "test-" + suffix
	if err := q.UpsertSource(ctx, store.UpsertSourceParams{
		ID: sourceID, Name: "Test store", Region: "US", Currency: "USD", Strategies: []string{"api"}, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Disable so later runs of the real worker never pick it up.
		_ = q.UpsertSource(context.Background(), store.UpsertSourceParams{
			ID: sourceID, Name: "Test store", Region: "US", Currency: "USD", Strategies: []string{"api"}, Enabled: false,
		})
	})
	return product, sourceID
}

func TestRefreshStoresOffersAndPriceHistory(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	product, sourceID := newProductAndSource(t, ctx, q)

	fake := &fakeStrategy{offers: []sources.Offer{
		{ExternalID: "a", URL: "https://example.test/a", Title: "GPU A", PriceMinor: 199900, Currency: "USD", Availability: "in_stock"},
		{ExternalID: "b", URL: "https://example.test/b", Title: "GPU B", PriceMinor: 209900, Currency: "USD"},
	}}
	adapters := Adapters{sourceID: {"api": fake}}

	runJob(t, ctx, pool, adapters, product.ID)
	assertCounts(t, ctx, q, product.ID, 2, 2)

	// Same prices again: offers are updated in place, no new price points.
	runJob(t, ctx, pool, adapters, product.ID)
	assertCounts(t, ctx, q, product.ID, 2, 2)

	// One price drops: one new price point, still two offers.
	fake.offers[0].PriceMinor = 189900
	runJob(t, ctx, pool, adapters, product.ID)
	assertCounts(t, ctx, q, product.ID, 2, 3)

	offers, err := q.ListOffersByProduct(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if offers[0].ExternalID != "a" || offers[0].PriceMinor != 189900 {
		t.Fatalf("cheapest offer: got %s at %d, want a at 189900", offers[0].ExternalID, offers[0].PriceMinor)
	}
	refreshed, err := q.GetProduct(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed.LastRefreshedAt.Valid {
		t.Fatal("last_refreshed_at not set")
	}
}

func TestRefreshSucceedsWhenASourceFails(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	product, goodID := newProductAndSource(t, ctx, q)
	_, badID := newProductAndSource(t, ctx, q)

	adapters := Adapters{
		goodID: {"api": &fakeStrategy{offers: []sources.Offer{
			{ExternalID: "only", URL: "https://example.test/only", Title: "GPU", PriceMinor: 100, Currency: "USD"},
		}}},
		badID: {"api": failingStrategy{}},
	}
	runJob(t, ctx, pool, adapters, product.ID)
	assertCounts(t, ctx, q, product.ID, 1, 1)
}

func assertCounts(t *testing.T, ctx context.Context, q *store.Queries, productID int64, wantOffers, wantPoints int) {
	t.Helper()
	offers, err := q.ListOffersByProduct(ctx, productID)
	if err != nil {
		t.Fatal(err)
	}
	points, err := q.CountPricePointsByProduct(ctx, productID)
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != wantOffers || int(points) != wantPoints {
		t.Fatalf("got %d offers and %d price points, want %d and %d", len(offers), points, wantOffers, wantPoints)
	}
}
