package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/db"
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

type fixture struct {
	q                  *store.Queries
	productID          int64
	sourceID           string
	cheap, dear, cable int64 // offer ids
	watchID            int64
}

// newFixture: one product with three CNY offers (¥500, ¥900, a ¥50 cable the
// matcher rejected) and a member tracking it with the given rules.
func newFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, rules []Rule, baseline int64) fixture {
	t.Helper()
	q := store.New(pool)
	suffix := fmt.Sprint(time.Now().UnixNano())
	p, err := q.UpsertProduct(ctx, store.UpsertProductParams{CanonicalName: "Alert Test " + suffix, Category: "gpu"})
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{q: q, productID: p.ID, sourceID: "alert-test-" + suffix}
	if err := q.UpsertSource(ctx, store.UpsertSourceParams{ID: f.sourceID, Name: "Test", Region: "CN", Currency: "CNY", Strategies: []string{}}); err != nil {
		t.Fatal(err)
	}
	add := func(ext string, price int64, isProduct bool) int64 {
		id, err := q.UpsertOffer(ctx, store.UpsertOfferParams{
			ProductID: p.ID, SourceID: f.sourceID, ExternalID: ext, Url: "https://example.test/" + ext, TitleOriginal: ext,
			PriceMinor: price, Currency: "CNY", Condition: "new", Availability: "in_stock", SellerType: "unknown",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.SetOfferMatch(ctx, store.SetOfferMatchParams{ID: id, IsProduct: &isProduct, TitleEn: ext, Condition: "new"}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	f.cheap, f.dear, f.cable = add("cheap", 50000, true), add("dear", 90000, true), add("cable", 5000, false)

	user := "alert-user-" + suffix
	if err := q.UpsertUser(ctx, store.UpsertUserParams{ID: user, Email: suffix + "@example.test"}); err != nil {
		t.Fatal(err)
	}
	// A tracker left behind would make a dev worker on this database refresh it.
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), user) })
	if err := q.SetUserChannels(ctx, store.SetUserChannelsParams{ID: user, FeishuWebhook: "https://open.feishu.cn/open-apis/bot/v2/hook/test", EmailAlerts: true}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(rules)
	w, err := q.SaveWatchItem(ctx, store.SaveWatchItemParams{UserID: user, ProductID: p.ID, Rules: raw, BaselinePriceCnyMinor: &baseline, Sources: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	f.watchID = w.ID
	return f
}

type queued struct{ ids []int64 }

func (q *queued) enqueue(_ context.Context, id int64) error {
	q.ids = append(q.ids, id)
	return nil
}

func TestRulesFireOnceForTheCheapestMatchingOffer(t *testing.T) {
	ctx, pool := setup(t)
	rules := []Rule{
		{Type: RuleBelow, Below: &Money{Minor: 60000, Currency: "CNY"}}, // ¥500 offer qualifies, the ¥50 cable must not
		{Type: RuleDrop, Percent: 10},                                   // baseline ¥600: ¥500 is a 17% drop
		{Type: RuleBelow, Below: &Money{Minor: 40000, Currency: "CNY"}}, // nothing that cheap
	}
	f := newFixture(t, ctx, pool, rules, 60000)
	out := &queued{}
	c := &Checker{EmailEnabled: false, Enqueue: out.enqueue} // Feishu only: no SMTP

	n, err := c.Check(ctx, f.q, f.productID, f.sourceID, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(out.ids) != 2 {
		t.Fatalf("alerts: %d, want 2 (below ¥600 and drop 10%%, Feishu only)", n)
	}
	for _, id := range out.ids {
		a, err := f.q.GetAlertToSend(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if a.Channel != ChannelFeishu || a.Url != "https://example.test/cheap" {
			t.Fatalf("alert %d: channel %s, offer %s", id, a.Channel, a.Url)
		}
	}

	// The same refresh again, or a later one today: no repeats (T5).
	if n, err := c.Check(ctx, f.q, f.productID, f.sourceID, Changes{}); err != nil || n != 0 {
		t.Fatalf("repeat: %d alerts, %v", n, err)
	}

	// With SMTP configured, email alerts join (they are a separate channel).
	c.EmailEnabled = true
	if n, err := c.Check(ctx, f.q, f.productID, f.sourceID, Changes{}); err != nil || n != 2 {
		t.Fatalf("email: %d alerts, %v", n, err)
	}
}

func TestStockAndNewSourceRulesNeedTheirChange(t *testing.T) {
	ctx, pool := setup(t)
	f := newFixture(t, ctx, pool, []Rule{{Type: RuleBackInStock}, {Type: RuleNewSource}}, 60000)
	out := &queued{}
	c := &Checker{Enqueue: out.enqueue}

	if n, _ := c.Check(ctx, f.q, f.productID, f.sourceID, Changes{}); n != 0 {
		t.Fatalf("no change: %d alerts", n)
	}
	// The rejected cable coming back in stock must not alert.
	if n, _ := c.Check(ctx, f.q, f.productID, f.sourceID, Changes{BackInStock: map[int64]bool{f.cable: true}}); n != 0 {
		t.Fatalf("accessory back in stock: %d alerts", n)
	}
	n, err := c.Check(ctx, f.q, f.productID, f.sourceID, Changes{BackInStock: map[int64]bool{f.dear: true}, FirstFromSource: true})
	if err != nil || n != 2 {
		t.Fatalf("back in stock and new source: %d alerts, %v", n, err)
	}
}

func TestValidateAndDescribe(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Fatal("empty rules accepted")
	}
	for _, bad := range []Rule{{Type: RuleBelow}, {Type: RuleDrop, Percent: 95}, {Type: "cheapest"}} {
		if Validate([]Rule{bad}) == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	format := func(m Money) string { return fmt.Sprintf("%d %s", m.Minor, m.Currency) }
	below := Rule{Type: RuleBelow, Below: &Money{Minor: 150000, Currency: "CNY"}}
	if got := DescribeKey(below.Key(), format); got != "price below 150000 CNY" {
		t.Fatalf("describe below: %q", got)
	}
	if got := DescribeKey(Rule{Type: RuleDrop, Percent: 15}.Key(), format); got != "price 15% below where it was when you started tracking" {
		t.Fatalf("describe drop: %q", got)
	}
}

func TestPinnedTrackerOnlyAlertsOnItsListing(t *testing.T) {
	ctx, pool := setup(t)
	// Below ¥1,000: both offers qualify, but the tracker follows the ¥900 one.
	f := newFixture(t, ctx, pool, []Rule{{Type: RuleBelow, Below: &Money{Minor: 100000, Currency: "CNY"}}}, 90000)
	if _, err := pool.Exec(ctx, "UPDATE app.watch_items SET offer_id = $1 WHERE id = $2", f.dear, f.watchID); err != nil {
		t.Fatal(err)
	}
	out := &queued{}
	n, err := (&Checker{Enqueue: out.enqueue}).Check(ctx, f.q, f.productID, f.sourceID, Changes{})
	if err != nil || n != 1 {
		t.Fatalf("alerts: %d, %v", n, err)
	}
	var offerID int64
	if err := pool.QueryRow(ctx, "SELECT offer_id FROM app.alerts WHERE id = $1", out.ids[0]).Scan(&offerID); err != nil {
		t.Fatal(err)
	}
	if offerID != f.dear {
		t.Fatalf("alert for offer %d, want the pinned %d (not the cheaper %d)", offerID, f.dear, f.cheap)
	}
}

func TestTrackersOfAVariantIgnoreOtherVariants(t *testing.T) {
	ctx, pool := setup(t)
	f := newFixture(t, ctx, pool, []Rule{{Type: RuleBelow, Below: &Money{Minor: 100000, Currency: "CNY"}}}, 90000)
	// The ¥500 offer is 24GB, the ¥900 one 48GB; the member tracks the 48GB variant.
	for id, v := range map[int64]string{f.cheap: "24GB", f.dear: "48GB"} {
		if _, err := pool.Exec(ctx, "UPDATE app.offers SET variant = $1 WHERE id = $2", v, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE app.watch_items SET variant = '48GB' WHERE id = $1", f.watchID); err != nil {
		t.Fatal(err)
	}
	out := &queued{}
	if n, err := (&Checker{Enqueue: out.enqueue}).Check(ctx, f.q, f.productID, f.sourceID, Changes{}); err != nil || n != 1 {
		t.Fatalf("alerts: %d, %v", n, err)
	}
	var offerID int64
	if err := pool.QueryRow(ctx, "SELECT offer_id FROM app.alerts WHERE id = $1", out.ids[0]).Scan(&offerID); err != nil {
		t.Fatal(err)
	}
	if offerID != f.dear {
		t.Fatalf("alert for offer %d, want the 48GB one %d", offerID, f.dear)
	}
}
