package search

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/matcher"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// HistoryDays is how far back the product page's price history goes.
const HistoryDays = 30

// DayLow is the lowest price seen on one UTC day, across all offers or of one listing.
type DayLow struct {
	Day       string `json:"day" format:"date"`
	Converted Money  `json:"converted" doc:"Lowest price that day, in the requested currency"`
}

// ProductPage is everything the product page shows.
type ProductPage struct {
	Product        Product   `json:"product"`
	LastRefreshed  time.Time `json:"last_refreshed,omitzero"`
	Refreshing     bool      `json:"refreshing" doc:"true while sources are being searched"`
	Offers         []Offer   `json:"offers" doc:"Cheapest first, compared in CNY"`
	History        []DayLow  `json:"history" doc:"Oldest first; days without any price are left out"`
	HistoryOf      int64     `json:"history_of,omitempty" doc:"Offer id the history is for"`
	HistoryVariant string    `json:"history_variant,omitempty" doc:"Variant the history is the daily lowest of, when no listing is given"`
}

// Product returns a product's offers and a price history: of one listing
// (listing > 0), or the daily lowest of one variant. Listings of different
// variants are not mixed into one line, so without either there is no history.
// It never starts a refresh, so opening the page costs nothing. pgx.ErrNoRows
// when unknown.
func (s *Service) Product(ctx context.Context, productID int64, currency string, listing int64, variant string) (ProductPage, error) {
	q := store.New(s.DB)
	p, err := q.GetProduct(ctx, productID)
	if err != nil {
		return ProductPage{}, err
	}
	page := ProductPage{
		Product:    Product{ID: p.ID, Name: p.CanonicalName},
		Refreshing: len(p.RefreshingSources) > 0,
		History:    []DayLow{},
	}
	if p.LastRefreshedAt.Valid {
		page.LastRefreshed = p.LastRefreshedAt.Time
	}
	if page.Offers, err = s.offers(ctx, q, p.ID, nil, currency); err != nil {
		return ProductPage{}, err
	}

	since := time.Now().UTC().AddDate(0, 0, -HistoryDays+1).Truncate(24 * time.Hour)
	from := pgtype.Timestamptz{Time: since, Valid: true}
	var points []store.ListProductPricePointsRow
	if listing > 0 {
		rows, err := q.ListOfferPricePoints(ctx, store.ListOfferPricePointsParams{OfferID: listing, ProductID: p.ID, Since: from})
		if err != nil {
			return ProductPage{}, err
		}
		for _, r := range rows {
			points = append(points, store.ListProductPricePointsRow(r))
		}
		page.HistoryOf = listing
	} else if variant = matcher.NormalizeVariant(variant); variant != "" {
		if points, err = q.ListProductPricePoints(ctx, store.ListProductPricePointsParams{ProductID: p.ID, Since: from, Variant: variant, SourceIds: []string{}}); err != nil {
			return ProductPage{}, err
		}
		page.HistoryVariant = variant
	}
	rates, err := fx.Latest(ctx, q)
	if err != nil {
		return page, nil // no rates yet: history cannot be compared across currencies
	}
	page.History = dailyLows(points, rates, currency, time.Now())
	return page, nil
}

// dailyLows reduces price points to the lowest price per UTC day, converted to
// currency, with days without a change filled up to until (see fx.DailyLows
// and fx.FillDays).
func dailyLows(points []store.ListProductPricePointsRow, rates fx.Rates, currency string, until time.Time) []DayLow {
	in := make([]fx.Point, len(points))
	for i, p := range points {
		in[i] = fx.Point{At: p.ObservedAt.Time, Minor: p.PriceMinor, Currency: p.Currency}
	}
	lows := fx.FillDays(rates.DailyLows(in), until)
	out := make([]DayLow, 0, len(lows))
	for _, d := range lows {
		if converted, err := rates.FromCNYMinor(d.CNY, currency); err == nil {
			out = append(out, DayLow{Day: d.Day, Converted: Money{Minor: converted, Currency: currency}})
		}
	}
	return out
}
