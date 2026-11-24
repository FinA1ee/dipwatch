package kakaku

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"

	"github.com/FinA1ee/dipwatch/core/internal/sources"
)

// serveFixture serves testdata/search.html encoded as Shift_JIS, like the real site.
func serveFixture(t *testing.T, gotPath *string) *httptest.Server {
	t.Helper()
	page, err := os.ReadFile("testdata/search.html")
	if err != nil {
		t.Fatal(err)
	}
	sjis, err := japanese.ShiftJIS.NewEncoder().Bytes(page)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "text/html; charset=shift_jis")
		_, _ = w.Write(sjis)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSearchParsesResultItems(t *testing.T) {
	var path string
	srv := serveFixture(t, &path)
	k := New(srv.Client(), srv.URL)

	offers, err := k.Search(context.Background(), sources.Query{Product: "RTX 5090", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/RTX%205090/" {
		t.Errorf("request path: got %q, want /RTX%%205090/", path)
	}
	if len(offers) != 2 {
		t.Fatalf("got %d offers, want 2 (the item without a price is skipped): %+v", len(offers), offers)
	}

	catalog := offers[0]
	if catalog.ExternalID != "K0001673984" ||
		catalog.URL != "https://kakaku.com/item/K0001673984/" ||
		catalog.Title != "MSI GeForce RTX 5090 32G VENTUS 3X OC [PCIExp 32GB]" ||
		catalog.PriceMinor != 1280000 || catalog.Currency != "JPY" ||
		catalog.Condition != "new" || catalog.Availability != "in_stock" {
		t.Errorf("catalog item: %+v", catalog)
	}

	mall := offers[1]
	if !strings.HasPrefix(mall.ExternalID, "mall-") || mall.PriceMinor != 1668981 ||
		mall.Title != "ASUS ROG Astral GeForce RTX 5090 32GB GDDR7 OC Edition" || mall.SellerType != "third_party" {
		t.Errorf("mall item: %+v", mall)
	}
}

func TestSearchRespectsLimit(t *testing.T) {
	var path string
	srv := serveFixture(t, &path)
	offers, err := New(srv.Client(), srv.URL).Search(context.Background(), sources.Query{Product: "RTX 5090", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 1 {
		t.Fatalf("got %d offers, want 1", len(offers))
	}
}

func TestSearchReportsBlockedResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := New(srv.Client(), srv.URL).Search(context.Background(), sources.Query{Product: "x", Limit: 5}); err == nil {
		t.Fatal("want error on 403")
	}
}
