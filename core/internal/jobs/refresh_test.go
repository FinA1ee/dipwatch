package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/llm"
	"github.com/FinA1ee/dipwatch/core/internal/matcher"
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

// runJob runs one product refresh on a dedicated River client and waits for the
// fan-out job and every per-source job (one per adapter) to finish.
func runJob(t *testing.T, ctx context.Context, pool *pgxpool.Pool, adapters Adapters, productID int64) {
	t.Helper()
	runJobWith(t, ctx, pool, adapters, nil, productID)
}

func runJobWith(t *testing.T, ctx context.Context, pool *pgxpool.Pool, adapters Adapters, m *matcher.Matcher, productID int64) {
	t.Helper()
	workers := river.NewWorkers()
	river.AddWorker(workers, &RefreshProductWorker{DB: pool, Adapters: adapters})
	river.AddWorker(workers, &RefreshSourceWorker{DB: pool, Registry: sources.NewRegistry(), Adapters: adapters, Matcher: m})
	// A queue of its own, so a worker from `make dev` sharing this database does not
	// take the test's jobs.
	queue := fmt.Sprintf("test_%d", time.Now().UnixNano())
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{queue: {MaxWorkers: 2}},
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

	if _, err := client.Insert(ctx, RefreshProductArgs{ProductID: productID}, &river.InsertOpts{Queue: queue}); err != nil {
		t.Fatal(err)
	}
	want := 1 + len(adapters)
	timeout := time.After(15 * time.Second)
	for finished := 0; finished < want; {
		select {
		case ev := <-done:
			if ev.Job.Queue != queue {
				continue // a job left over from another test run
			}
			if ev.Kind != river.EventKindJobCompleted {
				t.Fatalf("job %s %s: %v", ev.Job.Kind, ev.Kind, ev.Job.Errors)
			}
			finished++
		case <-timeout:
			t.Fatalf("refresh did not finish within 15s")
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
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+OffersChannel); err != nil {
		t.Fatal(err)
	}

	runJob(t, ctx, pool, adapters, product.ID)
	assertCounts(t, ctx, q, product.ID, 1, 1)

	// Both sources report as finished, the failed one with ok=false (S7).
	got := map[string]SourceDone{}
	for len(got) < 2 {
		waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		n, err := conn.Conn().WaitForNotification(waitCtx)
		cancel()
		if err != nil {
			t.Fatalf("notifications so far %+v: %v", got, err)
		}
		var d SourceDone
		if err := json.Unmarshal([]byte(n.Payload), &d); err != nil {
			t.Fatal(err)
		}
		if d.ProductID == product.ID {
			got[d.SourceID] = d
		}
	}
	if !got[goodID].OK || got[badID].OK || min(got[goodID].Remaining, got[badID].Remaining) != 0 {
		t.Fatalf("notifications: %+v", got)
	}
	refreshed, err := q.GetProduct(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed.RefreshingSources) != 0 || !refreshed.LastRefreshedAt.Valid {
		t.Fatalf("refresh state: running %v, refreshed %v", refreshed.RefreshingSources, refreshed.LastRefreshedAt.Valid)
	}
}

func TestRefreshStaysStaleWhenEverySourceFails(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	product, badID := newProductAndSource(t, ctx, q)
	runJob(t, ctx, pool, Adapters{badID: {"api": failingStrategy{}}}, product.ID)
	p, err := q.GetProduct(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.LastRefreshedAt.Valid {
		t.Fatal("a refresh where every source failed marked the product fresh")
	}
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

var numberedLine = regexp.MustCompile(`(?m)^(\d+)\. (.*)$`)

// noCables is a scripted matcher model: every listing is the product unless its
// title mentions a cable.
type noCables struct{ calls int }

func (m *noCables) Name() string                   { return "scripted" }
func (m *noCables) CostMicros(in, out int64) int64 { return in + out }
func (m *noCables) Chat(_ context.Context, msgs []llm.Message, _ []llm.Tool) (llm.Turn, error) {
	m.calls++
	var decisions []string
	for _, line := range numberedLine.FindAllStringSubmatch(msgs[len(msgs)-1].Content, -1) {
		decisions = append(decisions, fmt.Sprintf(`{"index":%s,"is_product":%t,"title_en":%q,"condition":"unknown"}`,
			line[1], !strings.Contains(line[2], "Cable"), line[2]))
	}
	return llm.Turn{ToolCalls: []llm.ToolCall{{ID: "1", Name: "submit_decisions",
		Arguments: `{"decisions":[` + strings.Join(decisions, ",") + `]}`}}}, nil
}

func TestRefreshMatchesListingsOnce(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	product, sourceID := newProductAndSource(t, ctx, q)
	fake := &fakeStrategy{offers: []sources.Offer{
		{ExternalID: "gpu", URL: "https://example.test/gpu", Title: "GPU 5090", PriceMinor: 100, Currency: "USD", Condition: "used"},
		{ExternalID: "cable", URL: "https://example.test/cable", Title: "Cable for GPU 5090", PriceMinor: 10, Currency: "USD"},
	}}
	model := &noCables{}
	adapters, m := Adapters{sourceID: {"api": fake}}, &matcher.Matcher{Model: model}

	runJobWith(t, ctx, pool, adapters, m, product.ID)
	runJobWith(t, ctx, pool, adapters, m, product.ID) // cached: no second LLM call
	if model.calls != 1 {
		t.Fatalf("LLM called %d times, want 1", model.calls)
	}
	offers, err := q.ListOffersByProduct(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]store.AppOffer{}
	for _, o := range offers {
		got[o.ExternalID] = o
	}
	gpu, cable := got["gpu"], got["cable"]
	if gpu.IsProduct == nil || !*gpu.IsProduct || cable.IsProduct == nil || *cable.IsProduct {
		t.Fatalf("is_product: gpu %v, cable %v", gpu.IsProduct, cable.IsProduct)
	}
	if gpu.Condition != "used" {
		t.Fatalf("source condition lost: %q", gpu.Condition)
	}
}
