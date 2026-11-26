package ecapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/FinA1ee/dipwatch/core/internal/sources"
)

// testdata/jd-rtx4090.json is a recorded answer for "RTX 4090 24G", trimmed to
// the fields the adapter reads; it holds no account identifiers.
func TestParseJD(t *testing.T) {
	body, err := os.ReadFile("testdata/jd-rtx4090.json")
	if err != nil {
		t.Fatal(err)
	}
	offers, err := parseJD(body, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 5 {
		t.Fatalf("%d offers, want 5", len(offers))
	}
	first := offers[0]
	if first.PriceMinor != 3536600 || first.Currency != "CNY" || first.SellerType != "official" ||
		!strings.HasPrefix(first.URL, "https://jingfen.jd.com/detail/") || first.ExternalID == "" {
		t.Fatalf("first offer: %+v", first)
	}
	if offers[1].PriceMinor != 3294118 { // ¥32,941.18
		t.Fatalf("second price: %d", offers[1].PriceMinor)
	}
	if offers[3].SellerType != "third_party" {
		t.Fatalf("a POP seller: %+v", offers[3])
	}
}

func TestParseJDUsedAndErrors(t *testing.T) {
	used := `{"code":200,"msg":"suc","data":{"list":[{"itemId":"a","skuName":"【99成新】索尼 WF-1000XM5","owner":"g",
		"materialUrl":"jingfen.jd.com/detail/a.html","priceInfo":{"price":899,"lowestCouponPrice":799},
		"purchasePriceInfo":{"code":200,"purchasePrice":779},"categoryInfo":{"cid3Name":"二手耳机/耳麦"}}]}}`
	offers, err := parseJD([]byte(used), 5)
	if err != nil || len(offers) != 1 || offers[0].Condition != "used" || offers[0].PriceMinor != 77900 {
		t.Fatalf("used: %+v, %v", offers, err)
	}
	// Without a purchase price, the lowest coupon price is what a shopper pays.
	noPurchase := strings.Replace(used, `"code":200,"purchasePrice":779`, `"code":500`, 1)
	if offers, _ := parseJD([]byte(noPurchase), 5); len(offers) != 1 || offers[0].PriceMinor != 79900 {
		t.Fatalf("coupon price: %+v", offers)
	}
	if _, err := parseJD([]byte(`{"code":400,"data":{},"msg":"key_id必须填写"}`), 5); err == nil || !strings.Contains(err.Error(), "key_id") {
		t.Fatalf("api error: %v", err)
	}
	if offers, err := parseJD([]byte(`{"code":200,"msg":"suc","data":{}}`), 5); err != nil || len(offers) != 0 {
		t.Fatalf("no results: %+v, %v", offers, err)
	}
}

func TestJDRequestAndKeyNotInErrors(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Path + "?" + r.URL.RawQuery
		_, _ = w.Write([]byte(`{"code":200,"msg":"suc","data":{"list":[]}}`))
	}))
	defer srv.Close()
	j := &JD{HTTP: srv.Client(), URL: srv.URL, APIKey: "secret-key", KeyID: "J1"}
	if _, err := j.Search(context.Background(), sources.Query{Product: "索尼 WF-1000XM5", Limit: 50}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/jingdong/getJdUnionItems?", "key_id=J1", "pageSize=30", "sortName=inOrderCount30Days"} {
		if !strings.Contains(got, want) {
			t.Errorf("request %q lacks %q", got, want)
		}
	}

	srv.Close() // now the request fails at the transport
	_, err := j.Search(context.Background(), sources.Query{Product: "x", Limit: 1})
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("transport error must not show the key: %v", err)
	}
}
