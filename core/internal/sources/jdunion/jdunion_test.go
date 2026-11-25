package jdunion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/sources"
)

// A response in the router's shape: the API's JSON as a string in the envelope.
func routerBody(t *testing.T, result any) string {
	t.Helper()
	inner, _ := json.Marshal(result)
	outer, _ := json.Marshal(map[string]any{
		"jd_union_open_goods_query_responce": map[string]any{"code": "0", "queryResult": string(inner)}, //nolint:misspell // JD's spelling
	})
	return string(outer)
}

func TestSignMatchesTheRouterScheme(t *testing.T) {
	// Worked by hand: secret + "a1" + "b2" + secret, MD5, upper-case hex.
	got := Sign("s", map[string]string{"b": "2", "a": "1", "sign": "ignored"})
	if got != "5EE29085AF57D942F21F1C5BA3C2A90A" { // md5("sa1b2s")
		t.Fatalf("sign %q", got)
	}
	if Sign("s", map[string]string{"a": "1", "b": "2"}) != got {
		t.Fatal("the sign parameter itself must not be signed")
	}
}

func TestSearchSendsASignedRequestAndParsesOffers(t *testing.T) {
	var form map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = map[string]string{}
		for k := range r.PostForm {
			form[k] = r.PostForm.Get(k)
		}
		_, _ = w.Write([]byte(routerBody(t, map[string]any{
			"code": 200, "message": "success", "totalCount": 3,
			"data": []map[string]any{
				{"skuId": 100012043978, "skuName": "索尼（SONY）WH-1000XM6 头戴式无线降噪耳机", "owner": "g",
					"materialUrl": "https://item.jd.com/100012043978.html",
					"priceInfo":   map[string]any{"price": 2999, "lowestPrice": 2799, "lowestCouponPrice": 2699.5},
					"shopInfo":    map[string]any{"shopName": "索尼京东自营官方旗舰店"}},
				{"skuId": 222, "skuName": "WH-1000XM6 耳罩", "owner": "p", "materialUrl": "",
					"priceInfo": map[string]any{"price": 59, "lowestPrice": 0, "lowestCouponPrice": 0}},
				{"skuId": 333, "skuName": "no price", "priceInfo": map[string]any{}},
			},
		})))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), URL: srv.URL, AppKey: "key", AppSecret: "secret",
		Now: func() time.Time { return time.Date(2026, 11, 25, 1, 2, 3, 0, time.UTC) }}
	offers, err := c.Search(context.Background(), sources.Query{Product: "Sony WH-1000XM6", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}

	if form["method"] != method || form["app_key"] != "key" || form["timestamp"] != "2026-11-25 09:02:03" {
		t.Fatalf("request: %v", form)
	}
	if !strings.Contains(form["360buy_param_json"], `"keyword":"Sony WH-1000XM6"`) || !strings.Contains(form["360buy_param_json"], `"pageSize":30`) {
		t.Fatalf("payload: %s", form["360buy_param_json"])
	}
	signed := map[string]string{}
	for k, v := range form {
		signed[k] = v
	}
	if form["sign"] != Sign("secret", signed) {
		t.Fatal("sign does not match the parameters sent")
	}

	if len(offers) != 2 {
		t.Fatalf("offers: %+v", offers)
	}
	first := offers[0]
	if first.ExternalID != "100012043978" || first.PriceMinor != 269950 || first.Currency != "CNY" || first.SellerType != "official" {
		t.Fatalf("first offer: %+v (price after coupon, self-run)", first)
	}
	if offers[1].URL != "https://item.jd.com/222.html" || offers[1].PriceMinor != 5900 || offers[1].SellerType != "third_party" {
		t.Fatalf("second offer: %+v (fallback URL and list price)", offers[1])
	}
}

func TestSearchReportsRouterErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error_response":{"code":"19","zh_desc":"无效的app_key","en_desc":"Invalid app key"}}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), URL: srv.URL, AppKey: "bad", AppSecret: "x"}
	if _, err := c.Search(context.Background(), sources.Query{Product: "x", Limit: 5}); err == nil || !strings.Contains(err.Error(), "Invalid app key") {
		t.Fatalf("error: %v", err)
	}
}
