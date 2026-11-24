package sources

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStrategy struct {
	name   string
	offers []Offer
	err    error
	calls  int
}

func (f *fakeStrategy) Name() string { return f.name }

func (f *fakeStrategy) Search(context.Context, Query) ([]Offer, error) {
	f.calls++
	return f.offers, f.err
}

var q = Query{Product: "RTX 5090", Limit: 50}

func offers(ids ...string) []Offer {
	out := make([]Offer, len(ids))
	for i, id := range ids {
		out[i] = Offer{ExternalID: id, PriceMinor: 100, Currency: "USD"}
	}
	return out
}

func TestFirstStrategyWithOffersWins(t *testing.T) {
	api := &fakeStrategy{name: "api", offers: offers("a")}
	agent := &fakeStrategy{name: "agent", offers: offers("b")}
	r := NewRegistry()

	res, err := r.Search(context.Background(), Source{ID: "s", Strategies: []Strategy{api, agent}}, q)
	if err != nil {
		t.Fatal(err)
	}
	if res.Strategy != "api" || len(res.Offers) != 1 || res.Offers[0].ExternalID != "a" {
		t.Fatalf("got %+v, want api's offer", res)
	}
	if agent.calls != 0 {
		t.Fatalf("agent called %d times, want 0", agent.calls)
	}
}

func TestErrorAndEmptyFallThrough(t *testing.T) {
	broken := &fakeStrategy{name: "api", err: errors.New("403")}
	empty := &fakeStrategy{name: "html"}
	agent := &fakeStrategy{name: "agent", offers: offers("x", "y")}
	r := NewRegistry()

	res, err := r.Search(context.Background(), Source{ID: "s", Strategies: []Strategy{broken, empty, agent}}, q)
	if err != nil {
		t.Fatal(err)
	}
	if res.Strategy != "agent" || len(res.Offers) != 2 {
		t.Fatalf("got %+v, want 2 offers from agent", res)
	}
}

func TestAllFailingReturnsError(t *testing.T) {
	r := NewRegistry()
	_, err := r.Search(context.Background(), Source{ID: "s", Strategies: []Strategy{
		&fakeStrategy{name: "api", err: errors.New("down")},
		&fakeStrategy{name: "agent", err: errors.New("budget")},
	}}, q)
	if err == nil {
		t.Fatal("want error when every strategy fails")
	}
}

func TestAllEmptyIsNotAnError(t *testing.T) {
	r := NewRegistry()
	res, err := r.Search(context.Background(), Source{ID: "s", Strategies: []Strategy{&fakeStrategy{name: "api"}}}, q)
	if err != nil || len(res.Offers) != 0 {
		t.Fatalf("got %+v, %v; want no offers and no error", res, err)
	}
}

func TestThreeFailuresMarkStrategyUnhealthyForAnHour(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	r := NewRegistry()
	r.now = func() time.Time { return now }

	broken := &fakeStrategy{name: "api", err: errors.New("403")}
	backup := &fakeStrategy{name: "agent", offers: offers("b")}
	src := Source{ID: "s", Strategies: []Strategy{broken, backup}}

	for range 3 {
		if _, err := r.Search(context.Background(), src, q); err != nil {
			t.Fatal(err)
		}
	}
	if broken.calls != 3 {
		t.Fatalf("broken called %d times, want 3", broken.calls)
	}

	// Unhealthy: skipped for the next hour.
	now = now.Add(59 * time.Minute)
	_, _ = r.Search(context.Background(), src, q)
	if broken.calls != 3 {
		t.Fatalf("unhealthy strategy was called again (calls=%d)", broken.calls)
	}

	// After an hour it gets another chance; a success resets the count.
	now = now.Add(2 * time.Minute)
	broken.err, broken.offers = nil, offers("a")
	res, _ := r.Search(context.Background(), src, q)
	if broken.calls != 4 || res.Strategy != "api" {
		t.Fatalf("after cooldown: calls=%d strategy=%s, want 4 and api", broken.calls, res.Strategy)
	}
}

func TestHealthIsTrackedPerSource(t *testing.T) {
	r := NewRegistry()
	broken := &fakeStrategy{name: "api", err: errors.New("403")}
	for range 3 {
		_, _ = r.Search(context.Background(), Source{ID: "a", Strategies: []Strategy{broken}}, q)
	}
	_, _ = r.Search(context.Background(), Source{ID: "b", Strategies: []Strategy{broken}}, q)
	if broken.calls != 4 {
		t.Fatalf("source b should not inherit source a's unhealthy mark (calls=%d)", broken.calls)
	}
}
