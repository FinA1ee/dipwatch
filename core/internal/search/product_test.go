package search

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

func TestDailyLowsComparesInCNY(t *testing.T) {
	at := func(day, hour int) pgtype.Timestamptz {
		return pgtype.Timestamptz{Time: time.Date(2026, 11, day, hour, 0, 0, 0, time.UTC), Valid: true}
	}
	rates := fx.Rates{Rates: map[string]float64{"USD": 0.125}} // ¥8 per dollar
	points := []store.ListProductPricePointsRow{
		{ObservedAt: at(24, 1), PriceMinor: 90000, Currency: "CNY"}, // ¥900
		{ObservedAt: at(24, 9), PriceMinor: 10000, Currency: "USD"}, // $100 = ¥800, the low
		{ObservedAt: at(25, 3), PriceMinor: 85000, Currency: "CNY"},
		{ObservedAt: at(25, 4), PriceMinor: 100, Currency: "XXX"}, // no rate: ignored
	}
	got := dailyLows(points, rates, "USD")
	if len(got) != 2 || got[0].Day != "2026-11-24" || got[0].Converted.Minor != 10000 || got[1].Converted.Minor != 10625 {
		t.Fatalf("lows: %+v", got)
	}
}
