package tracking

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/alerts"
	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/guard"
	"github.com/FinA1ee/dipwatch/core/internal/notify"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

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

func products(t *testing.T, ctx context.Context, q *store.Queries, n int) []int64 {
	t.Helper()
	suffix := fmt.Sprint(time.Now().UnixNano())
	ids := make([]int64, n)
	for i := range ids {
		p, err := q.UpsertProduct(ctx, store.UpsertProductParams{CanonicalName: fmt.Sprintf("Track Test %s %d", suffix, i)})
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = p.ID
	}
	return ids
}

var anyStock = []alerts.Rule{{Type: alerts.RuleBackInStock}}

func TestTrackingLimitsAndUpdates(t *testing.T) {
	ctx, pool := setup(t)
	s := &Service{DB: pool} // default limits: 3 products, 2 stores, invite-only
	id := fmt.Sprintf("track-%d", time.Now().UnixNano())
	v := guard.Viewer{UserID: id, Email: id + "@example.test"}
	if _, err := s.Save(ctx, v, products(t, ctx, store.New(pool), 1)[0], "", anyStock, []string{"kakaku"}, nil); !errors.Is(err, guard.ErrInviteOnly) {
		t.Fatalf("uninvited member: %v", err)
	}
	v.Admin = true // admins need no invite
	t.Cleanup(func() { _ = store.New(pool).DeleteUser(context.Background(), id) })
	ids := products(t, ctx, store.New(pool), 4)
	at := []string{"kakaku"}

	if _, err := s.Save(ctx, guard.Viewer{}, ids[0], "", anyStock, at, nil); !errors.Is(err, ErrNotMember) {
		t.Fatalf("visitor: %v", err)
	}
	if _, err := s.Save(ctx, v, ids[0], strings.Repeat("备", MaxNoteRune+1), anyStock, at, nil); !errors.Is(err, ErrNoteTooLong) {
		t.Fatalf("long note: %v", err)
	}
	if _, err := s.Save(ctx, v, ids[0], "", []alerts.Rule{{Type: alerts.RuleDrop}}, at, nil); err == nil {
		t.Fatal("invalid rule accepted")
	}
	if _, err := s.Save(ctx, v, ids[0], "", anyStock, nil, nil); !errors.Is(err, ErrStores) {
		t.Fatalf("no store: %v", err)
	}
	if _, err := s.Save(ctx, v, ids[0], "", anyStock, []string{"kakaku", "smzdm", "jd"}, nil); !errors.Is(err, ErrStores) {
		t.Fatalf("three stores with a limit of two: %v", err)
	}
	if _, err := s.Save(ctx, v, ids[0], strings.Repeat("备", MaxNoteRune), anyStock, at, nil); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids[1:3] {
		if _, err := s.Save(ctx, v, id, "", anyStock, at, nil); err != nil {
			t.Fatal(err)
		}
	}
	var limit *LimitError
	if _, err := s.Save(ctx, v, ids[3], "", anyStock, at, nil); !errors.As(err, &limit) || limit.Limit != 3 {
		t.Fatalf("fourth product: %v (limit 3)", err)
	}
	// Changing an existing tracker is not a new one, and keeps its store.
	item, err := s.Save(ctx, v, ids[0], "upgrade", []alerts.Rule{{Type: alerts.RuleDrop, Percent: 10}}, nil, nil)
	if err != nil || item.Note != "upgrade" || item.Rules[0].Percent != 10 || len(item.Stores) != 1 || item.Stores[0] != "kakaku" {
		t.Fatalf("update: %+v, %v", item, err)
	}
	if err := s.Delete(ctx, v, ids[0]); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, v, ids[0]); !errors.Is(err, ErrNotTracked) {
		t.Fatalf("second delete: %v", err)
	}
	if _, err := s.Save(ctx, v, ids[3], "", anyStock, at, nil); err != nil {
		t.Fatalf("after stopping the first: %v", err)
	}
}

func TestSettingsCheckTheWebhookHost(t *testing.T) {
	ctx, pool := setup(t)
	s := &Service{DB: pool}
	id := fmt.Sprintf("settings-%d", time.Now().UnixNano())
	v := guard.Viewer{UserID: id, Email: id + "@example.test"}
	t.Cleanup(func() { _ = store.New(pool).DeleteUser(context.Background(), id) })

	if _, err := s.SaveSettings(ctx, v, "https://evil.example/open-apis/bot/v2/hook/x", true); !errors.Is(err, notify.ErrWebhookHost) {
		t.Fatalf("other host: %v (T6)", err)
	}
	if err := s.TestFeishu(ctx, v); !errors.Is(err, ErrNoWebhook) {
		t.Fatalf("test without webhook: %v", err)
	}
	set, err := s.SaveSettings(ctx, v, "https://open.feishu.cn/open-apis/bot/v2/hook/abc", false)
	if err != nil || set.FeishuWebhook == "" || set.EmailAlerts {
		t.Fatalf("saved: %+v, %v", set, err)
	}
}

func TestPinningAListing(t *testing.T) {
	ctx, pool := setup(t)
	s := &Service{DB: pool}
	id := fmt.Sprintf("pin-%d", time.Now().UnixNano())
	v := guard.Viewer{UserID: id, Email: id + "@example.test", Admin: true}
	t.Cleanup(func() { _ = store.New(pool).DeleteUser(context.Background(), id) })
	q := store.New(pool)
	ids := products(t, ctx, q, 2)
	offer := func(product int64, ext string, price int64) int64 {
		oid, err := q.UpsertOffer(ctx, store.UpsertOfferParams{
			ProductID: product, SourceID: "smzdm", ExternalID: ext, Url: "https://example.test/" + ext, TitleOriginal: ext,
			PriceMinor: price, Currency: "CNY", Condition: "new", Availability: "in_stock", SellerType: "unknown",
		})
		if err != nil {
			t.Fatal(err)
		}
		return oid
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	cheap := offer(ids[0], "pin-cheap-"+suffix, 1_200_000)
	founders := offer(ids[0], "pin-fe-"+suffix, 1_499_900)
	other := offer(ids[1], "pin-other-"+suffix, 100)

	if _, err := s.Save(ctx, v, ids[0], "", anyStock, nil, &other); !errors.Is(err, ErrListing) {
		t.Fatalf("another product's listing: %v", err)
	}
	// Pinning needs no store choice: the listing's store is the tracker's store,
	// and the baseline is the listing's price, not the product's cheapest.
	item, err := s.Save(ctx, v, ids[0], "", anyStock, nil, &founders)
	if err != nil {
		t.Fatal(err)
	}
	if item.Listing == nil || item.Listing.OfferID != founders || item.Listing.Gone ||
		len(item.Stores) != 1 || item.Stores[0] != "smzdm" || item.Baseline == nil || item.Baseline.Minor != 1_499_900 {
		t.Fatalf("pinned: %+v listing %+v", item, item.Listing)
	}
	// Unpinning goes back to the cheapest offer at the same store.
	item, err = s.Save(ctx, v, ids[0], "", anyStock, nil, nil)
	if err != nil || item.Listing != nil || item.Baseline == nil || item.Baseline.Minor != 1_200_000 {
		t.Fatalf("unpinned: %+v, %v", item, err)
	}
	_ = cheap
}
