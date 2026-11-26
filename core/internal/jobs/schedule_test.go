package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/store"
)

func TestAdaptiveIntervalAndDueProducts(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	tracked, _ := newProductAndSource(t, ctx, q)
	untracked, _ := newProductAndSource(t, ctx, q)
	user := "schedule-" + tracked.CanonicalName
	if err := q.UpsertUser(ctx, store.UpsertUserParams{ID: user, Email: time.Now().Format("150405.000000") + "@example.test"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), user) })
	if _, err := q.SaveWatchItem(ctx, store.SaveWatchItemParams{UserID: user, ProductID: tracked.ID, Rules: []byte(`[]`), Sources: []string{}}); err != nil {
		t.Fatal(err)
	}

	due := func() map[int64]bool {
		ids, err := q.DueTrackedProducts(ctx, 3600)
		if err != nil {
			t.Fatal(err)
		}
		m := map[int64]bool{}
		for _, row := range ids {
			m[row.ID] = true
		}
		return m
	}
	if d := due(); !d[tracked.ID] || d[untracked.ID] {
		t.Fatalf("never refreshed: tracked due %v, untracked due %v (want true, false)", d[tracked.ID], d[untracked.ID])
	}
	if err := q.MarkProductRefreshed(ctx, tracked.ID); err != nil {
		t.Fatal(err)
	}
	if due()[tracked.ID] {
		t.Fatal("due right after a refresh")
	}

	interval := func() time.Duration {
		p, err := q.GetProduct(ctx, tracked.ID)
		if err != nil {
			t.Fatal(err)
		}
		return time.Duration(p.RefreshInterval.Microseconds) * time.Microsecond
	}
	// A price change halves the interval, down to 2 hours.
	for _, want := range []time.Duration{3 * time.Hour, 2 * time.Hour, 2 * time.Hour} {
		_ = q.MarkRefreshChanged(ctx, tracked.ID)
		_ = q.AdaptRefreshInterval(ctx, tracked.ID)
		if got := interval(); got != want {
			t.Fatalf("after a change: %s, want %s", got, want)
		}
	}
	// Four refreshes without a change double it.
	for range 4 {
		_ = q.AdaptRefreshInterval(ctx, tracked.ID)
	}
	if got := interval(); got != 4*time.Hour {
		t.Fatalf("after 4 unchanged: %s, want 4h", got)
	}
	for range 4 * 3 {
		_ = q.AdaptRefreshInterval(ctx, tracked.ID)
	}
	if got := interval(); got != 24*time.Hour {
		t.Fatalf("capped at: %s, want 24h", got)
	}

	// A tracker that asks for hourly checks makes the product due after an hour,
	// although the automatic interval is 24 hours now.
	if _, err := pool.Exec(ctx, "UPDATE app.watch_items SET check_every = interval '1 hour' WHERE product_id = $1", tracked.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE app.products SET last_refreshed_at = now() - interval '90 minutes' WHERE id = $1", tracked.ID); err != nil {
		t.Fatal(err)
	}
	if !due()[tracked.ID] {
		t.Fatal("not due 90 minutes after a refresh with hourly checks")
	}
	// The owner's minimum wins over the member's choice.
	if ids, err := q.DueTrackedProducts(ctx, 2*3600); err != nil {
		t.Fatal(err)
	} else {
		for _, row := range ids {
			if row.ID == tracked.ID {
				t.Fatal("due before the owner's 2-hour minimum")
			}
		}
	}
}
