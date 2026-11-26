// Package fx converts prices to CNY with daily reference rates (spec R3, R4).
// Rates come from the Frankfurter API (European Central Bank data), are stored
// once a day, and the latest stored rates are used when the API is unreachable.
package fx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FinA1ee/dipwatch/core/internal/money"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// Base is the currency every price is ranked in.
const Base = "CNY"

// DefaultURL serves rates against CNY. The older api.frankfurter.app host
// redirects here.
const DefaultURL = "https://api.frankfurter.dev/v1/latest?base=" + Base

// Rates holds how much of each currency one CNY buys, e.g. USD: 0.138.
type Rates struct {
	Date  time.Time
	Rates map[string]float64
}

// ErrUnknownCurrency means no rate is stored for a currency.
var ErrUnknownCurrency = errors.New("fx: no rate for currency")

// ToCNYMinor converts an amount in minor units of currency to CNY fen.
func (r Rates) ToCNYMinor(amountMinor int64, currency string) (int64, error) {
	currency = strings.ToUpper(currency)
	if currency == Base {
		return amountMinor, nil
	}
	rate, ok := r.Rates[currency]
	if !ok || rate <= 0 {
		return 0, fmt.Errorf("%w %s", ErrUnknownCurrency, currency)
	}
	major := float64(amountMinor) / math.Pow10(money.Exponent(currency))
	return int64(math.Round(major / rate * 100)), nil
}

// FromCNYMinor converts CNY fen to minor units of currency, for display.
func (r Rates) FromCNYMinor(fen int64, currency string) (int64, error) {
	currency = strings.ToUpper(currency)
	if currency == Base {
		return fen, nil
	}
	rate, ok := r.Rates[currency]
	if !ok || rate <= 0 {
		return 0, fmt.Errorf("%w %s", ErrUnknownCurrency, currency)
	}
	major := float64(fen) / 100 * rate
	return int64(math.Round(major * math.Pow10(money.Exponent(currency)))), nil
}

// Fetch downloads today's rates.
func Fetch(ctx context.Context, client *http.Client, url string) (Rates, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Rates{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Rates{}, fmt.Errorf("fx: fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Rates{}, fmt.Errorf("fx: fetch: HTTP %d", resp.StatusCode)
	}
	var body struct {
		Base  string             `json:"base"`
		Date  string             `json:"date"`
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return Rates{}, fmt.Errorf("fx: decode: %w", err)
	}
	if body.Base != Base || len(body.Rates) == 0 {
		return Rates{}, fmt.Errorf("fx: unexpected response (base %q, %d rates)", body.Base, len(body.Rates))
	}
	date, err := time.Parse(time.DateOnly, body.Date)
	if err != nil {
		return Rates{}, fmt.Errorf("fx: date %q: %w", body.Date, err)
	}
	return Rates{Date: date, Rates: body.Rates}, nil
}

// Save stores rates for their date.
func Save(ctx context.Context, q *store.Queries, r Rates) error {
	raw, err := json.Marshal(r.Rates)
	if err != nil {
		return err
	}
	return q.UpsertFXRates(ctx, store.UpsertFXRatesParams{
		Date:  pgtype.Date{Time: r.Date, Valid: true},
		Base:  Base,
		Rates: raw,
	})
}

// Latest loads the most recent stored rates.
func Latest(ctx context.Context, q *store.Queries) (Rates, error) {
	row, err := q.LatestFXRates(ctx, Base)
	if err != nil {
		return Rates{}, fmt.Errorf("fx: no stored rates: %w", err)
	}
	r := Rates{Date: row.Date.Time}
	if err := json.Unmarshal(row.Rates, &r.Rates); err != nil {
		return Rates{}, err
	}
	return r, nil
}

// Refresh fetches today's rates and stores them. If the fetch fails, it returns
// the latest stored rates with the fetch error, so callers can log and continue.
func Refresh(ctx context.Context, q *store.Queries, client *http.Client, url string) (Rates, error) {
	r, fetchErr := Fetch(ctx, client, url)
	if fetchErr == nil {
		if err := Save(ctx, q, r); err != nil {
			return r, err
		}
		return r, nil
	}
	stored, err := Latest(ctx, q)
	if err != nil {
		return Rates{}, errors.Join(fetchErr, err)
	}
	return stored, fetchErr
}

// Point is one observed price.
type Point struct {
	At       time.Time
	Minor    int64
	Currency string
}

// DayLow is the lowest price of one UTC day, in CNY minor units.
type DayLow struct {
	Day string // YYYY-MM-DD
	CNY int64
}

// DailyLows reduces points (oldest first) to the lowest CNY value per UTC day.
// Today's rates are used for every day, so a line drawn from them shows price
// moves rather than exchange-rate moves. Points without a rate are left out.
func (r Rates) DailyLows(points []Point) []DayLow {
	var out []DayLow
	for _, p := range points {
		cny, err := r.ToCNYMinor(p.Minor, p.Currency)
		if err != nil {
			continue
		}
		day := p.At.UTC().Format(time.DateOnly)
		if n := len(out); n > 0 && out[n-1].Day == day {
			out[n-1].CNY = min(out[n-1].CNY, cny)
			continue
		}
		out = append(out, DayLow{Day: day, CNY: cny})
	}
	return out
}
