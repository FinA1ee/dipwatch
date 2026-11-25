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
	s := &Service{DB: pool} // default limits: 1 product, 1 store
	id := fmt.Sprintf("track-%d", time.Now().UnixNano())
	v := guard.Viewer{UserID: id, Email: id + "@example.test"}
	t.Cleanup(func() { _ = store.New(pool).DeleteUser(context.Background(), id) })
	ids := products(t, ctx, store.New(pool), 2)
	at := []string{"kakaku"}

	if _, err := s.Save(ctx, guard.Viewer{}, ids[0], "", anyStock, at); !errors.Is(err, ErrNotMember) {
		t.Fatalf("visitor: %v", err)
	}
	if _, err := s.Save(ctx, v, ids[0], strings.Repeat("备", MaxNoteRune+1), anyStock, at); !errors.Is(err, ErrNoteTooLong) {
		t.Fatalf("long note: %v", err)
	}
	if _, err := s.Save(ctx, v, ids[0], "", []alerts.Rule{{Type: alerts.RuleDrop}}, at); err == nil {
		t.Fatal("invalid rule accepted")
	}
	if _, err := s.Save(ctx, v, ids[0], "", anyStock, nil); !errors.Is(err, ErrStores) {
		t.Fatalf("no store: %v", err)
	}
	if _, err := s.Save(ctx, v, ids[0], "", anyStock, []string{"kakaku", "smzdm"}); !errors.Is(err, ErrStores) {
		t.Fatalf("two stores with a limit of one: %v", err)
	}
	if _, err := s.Save(ctx, v, ids[0], strings.Repeat("备", MaxNoteRune), anyStock, at); err != nil {
		t.Fatal(err)
	}
	var limit *LimitError
	if _, err := s.Save(ctx, v, ids[1], "", anyStock, at); !errors.As(err, &limit) || limit.Limit != 1 {
		t.Fatalf("second product: %v (limit 1)", err)
	}
	// Changing an existing tracker is not a new one, and keeps its store.
	item, err := s.Save(ctx, v, ids[0], "upgrade", []alerts.Rule{{Type: alerts.RuleDrop, Percent: 10}}, nil)
	if err != nil || item.Note != "upgrade" || item.Rules[0].Percent != 10 || len(item.Stores) != 1 || item.Stores[0] != "kakaku" {
		t.Fatalf("update: %+v, %v", item, err)
	}
	if err := s.Delete(ctx, v, ids[0]); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, v, ids[0]); !errors.Is(err, ErrNotTracked) {
		t.Fatalf("second delete: %v", err)
	}
	if _, err := s.Save(ctx, v, ids[1], "", anyStock, at); err != nil {
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
