// Package pricetrust marks prices that should not be compared: far from the
// other listings of the same variant (a reseller's ¥2,288,800 RTX 4090 among
// ¥15,000 ones), or relayed by the store without being checked (see
// sources.Offer.PriceUnconfirmed). Marked offers stay visible with a note but
// are not the "lowest", not on the price ladder and never trigger an alert.
package pricetrust

import "slices"

// Factor is how far from the variant's median a price may be: more than
// Factor times the median, or less than the median divided by Factor, is unusual.
const Factor = 2

// MinGroup is the fewest prices a variant needs before any of them can be
// called unusual: with two, there is no telling which one is off.
const MinGroup = 3

// Price is one listing's price in CNY minor units, with its variant.
type Price struct {
	CNY     int64
	Known   bool   // false when there is no exchange rate for its currency
	Variant string // "" counts as its own group
}

// Unusual reports, for each price, whether it is far from the median of the
// known prices of its variant.
func Unusual(prices []Price) []bool {
	groups := map[string][]int64{}
	for _, p := range prices {
		if p.Known {
			groups[p.Variant] = append(groups[p.Variant], p.CNY)
		}
	}
	medians := map[string]int64{}
	for v, list := range groups {
		if len(list) >= MinGroup {
			slices.Sort(list)
			medians[v] = list[len(list)/2]
		}
	}
	out := make([]bool, len(prices))
	for i, p := range prices {
		m, ok := medians[p.Variant]
		if ok && p.Known && m > 0 {
			out[i] = p.CNY > m*Factor || p.CNY*Factor < m
		}
	}
	return out
}
