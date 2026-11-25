package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/FinA1ee/dipwatch/core/internal/notify"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// feishuServer answers like Feishu at open.feishu.cn, with the given code.
func feishuServer(t *testing.T, code *int, got *[]string) notify.Feishu {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Content struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		*got = append(*got, body.Content.Text)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": *code, "msg": "test"})
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	transport := srv.Client().Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, target.Host)
	}
	transport.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // test server certificate
	return notify.Feishu{Client: &http.Client{Transport: transport}}
}

func TestNotifySendsOnceAndGivesUpOnARefusal(t *testing.T) {
	ctx, pool := setup(t)
	q := store.New(pool)
	product, sourceID := newProductAndSource(t, ctx, q)
	offerID, err := q.UpsertOffer(ctx, store.UpsertOfferParams{
		ProductID: product.ID, SourceID: sourceID, ExternalID: "x", Url: "https://example.test/x", TitleOriginal: "GPU X",
		PriceMinor: 149900, Currency: "USD", Condition: "new", Availability: "in_stock", SellerType: "unknown",
	})
	if err != nil {
		t.Fatal(err)
	}
	user := fmt.Sprintf("notify-%d", time.Now().UnixNano())
	if err := q.UpsertUser(ctx, store.UpsertUserParams{ID: user, Email: user + "@example.test"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), user) })
	_ = q.SetUserChannels(ctx, store.SetUserChannelsParams{ID: user, FeishuWebhook: "https://open.feishu.cn/open-apis/bot/v2/hook/t"})
	w, err := q.SaveWatchItem(ctx, store.SaveWatchItemParams{UserID: user, ProductID: product.ID, Rules: []byte(`[]`), Sources: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	alert := func(rule string) int64 {
		id, err := q.InsertAlert(ctx, store.InsertAlertParams{WatchItemID: w.ID, OfferID: offerID, Rule: rule, Channel: "feishu"})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	code := 0
	var got []string
	worker := &NotifyWorker{DB: pool, Feishu: feishuServer(t, &code, &got), PublicURL: "http://site.test"}
	job := func(id int64) *river.Job[NotifyArgs] {
		return &river.Job[NotifyArgs]{JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 5}, Args: NotifyArgs{AlertID: id}}
	}

	sent := alert("drop:10")
	if err := worker.Work(ctx, job(sent)); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "1499.00 USD") || !strings.Contains(got[0], "10% below") ||
		!strings.Contains(got[0], fmt.Sprintf("http://site.test/p/%d", product.ID)) {
		t.Fatalf("message: %q", got)
	}
	// A retry after success sends nothing.
	if err := worker.Work(ctx, job(sent)); err != nil || len(got) != 1 {
		t.Fatalf("resent: %d messages, %v", len(got), err)
	}

	code = 19001 // bad token: retrying cannot help
	refused := alert("back_in_stock")
	err = worker.Work(ctx, job(refused))
	var cancel *river.JobCancelError
	if !errors.As(err, &cancel) {
		t.Fatalf("refusal: %v, want the job cancelled", err)
	}
	row, err := q.GetAlertToSend(ctx, refused)
	if err != nil || row.Status != "failed" {
		t.Fatalf("status after refusal: %q, %v", row.Status, err)
	}
}
