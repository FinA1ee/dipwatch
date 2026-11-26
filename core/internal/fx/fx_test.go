package fx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

var sample = Rates{Rates: map[string]float64{"USD": 0.14, "SGD": 0.18, "JPY": 21.5}}

func TestConversion(t *testing.T) {
	tests := []struct {
		minor    int64
		currency string
		wantFen  int64
	}{
		{199999, "USD", 1428564},  // 1999.99 USD / 0.14 = 14285.64 CNY
		{1516075, "SGD", 8422639}, // 15160.75 SGD / 0.18
		{298000, "JPY", 1386047},  // 298000 JPY / 21.5 = 13860.47 CNY
		{3999900, "CNY", 3999900},
		{100, "usd", 714},
	}
	for _, tt := range tests {
		got, err := sample.ToCNYMinor(tt.minor, tt.currency)
		if err != nil || got != tt.wantFen {
			t.Errorf("ToCNYMinor(%d %s) = %d, %v; want %d", tt.minor, tt.currency, got, err, tt.wantFen)
		}
	}
	if _, err := sample.ToCNYMinor(100, "EUR"); !errors.Is(err, ErrUnknownCurrency) {
		t.Errorf("EUR: got %v, want ErrUnknownCurrency", err)
	}
	back, err := sample.FromCNYMinor(1428564, "USD")
	if err != nil || back != 199999 {
		t.Errorf("FromCNYMinor round trip: %d, %v", back, err)
	}
}

func TestFetchParsesTheAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("base") != "CNY" {
			t.Errorf("base query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"amount":1.0,"base":"CNY","date":"2026-11-24","rates":{"USD":0.1404,"JPY":21.62,"SGD":0.1822}}`))
	}))
	defer srv.Close()

	r, err := Fetch(context.Background(), srv.Client(), srv.URL+"/v1/latest?base=CNY")
	if err != nil {
		t.Fatal(err)
	}
	if r.Date.Format(time.DateOnly) != "2026-11-24" || r.Rates["USD"] != 0.1404 || len(r.Rates) != 3 {
		t.Fatalf("parsed %+v", r)
	}
}

func TestRefreshFallsBackToStoredRates(t *testing.T) {
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
	defer pool.Close()
	q := store.New(pool)

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"base":"CNY","date":"2099-01-02","rates":{"USD":0.25}}`))
	}))
	defer ok.Close()
	if _, err := Refresh(ctx, q, ok.Client(), ok.URL); err != nil {
		t.Fatal(err)
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer down.Close()
	r, err := Refresh(ctx, q, down.Client(), down.URL)
	if err == nil {
		t.Fatal("want the fetch error to be reported")
	}
	if r.Rates["USD"] != 0.25 || r.Date.Format(time.DateOnly) != "2099-01-02" {
		t.Fatalf("fallback rates: %+v", r)
	}
	// Leave no far-future row behind for other code that reads the latest rates.
	_, _ = pool.Exec(ctx, `DELETE FROM app.fx_rates WHERE date = '2099-01-02'`)
}

func TestDailyLows(t *testing.T) {
	r := Rates{Rates: map[string]float64{"JPY": 21}} // 1 CNY = 21 JPY
	day := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, time.UTC) }
	got := r.DailyLows([]Point{
		{At: day(24, 1), Minor: 120000, Currency: "CNY"},
		{At: day(24, 9), Minor: 110000, Currency: "CNY"}, // lower the same day
		{At: day(25, 3), Minor: 21000, Currency: "JPY"}, // ¥1,000 in CNY
		{At: day(25, 4), Minor: 5, Currency: "XXX"},     // no rate: left out
	})
	want := []DayLow{{Day: "2026-11-24", CNY: 110000}, {Day: "2026-11-25", CNY: 100000}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
