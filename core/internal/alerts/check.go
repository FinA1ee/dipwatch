package alerts

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/pricetrust"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// Changes is what one source refresh changed, collected while saving offers.
type Changes struct {
	BackInStock     map[int64]bool // offers that were out of stock and are in stock now
	FirstFromSource bool           // the product had no offer from this source before
	Changed         bool           // a known offer's price or stock changed (for the adaptive interval)
}

// Channels a tracker's alerts go to.
const (
	ChannelFeishu = "feishu"
	ChannelEmail  = "email"
)

// Checker evaluates rules and writes alerts.
type Checker struct {
	// EmailEnabled is true when SMTP is configured; otherwise no email alerts are written.
	EmailEnabled bool
	// Enqueue starts delivery of a new alert.
	Enqueue func(ctx context.Context, alertID int64) error
}

type offer struct {
	id      int64
	source  string
	cny     int64
	known   bool // has an exchange rate, and its price can be compared (pricetrust)
	variant string
}

// Check runs every tracker's rules after one source's refresh of a product
// (T1, T4). Each rule fires for the cheapest matching offer of the tracker's
// variant, or only for the pinned listing when the tracker has one, at most
// once per offer, rule and channel per day (T5, enforced by the alerts table).
// Offers whose price is unconfirmed or unusual never trigger an alert.
//
// Price rules look at the refreshed source's offers. Rule "change" compares
// the tracked price across all of the tracker's stores with the last one it
// saw, so a tracker of two stores does not flip between them.
func (c *Checker) Check(ctx context.Context, q *store.Queries, productID int64, sourceID string, ch Changes) (int, error) {
	watchers, err := q.ListProductWatchers(ctx, productID)
	if err != nil || len(watchers) == 0 {
		return 0, err
	}
	rows, err := q.ListAlertOffers(ctx, productID)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	rates, err := fx.Latest(ctx, q)
	if err != nil {
		rates = fx.Rates{} // CNY offers can still be compared
	}
	offers := make([]offer, 0, len(rows))
	prices := make([]pricetrust.Price, 0, len(rows))
	for _, r := range rows {
		cny, err := rates.ToCNYMinor(r.PriceMinor, r.Currency)
		known := err == nil && !r.PriceUnconfirmed
		offers = append(offers, offer{id: r.ID, source: r.SourceID, cny: cny, known: known, variant: r.Variant})
		prices = append(prices, pricetrust.Price{CNY: cny, Known: known, Variant: r.Variant})
	}
	for i, unusual := range pricetrust.Unusual(prices) {
		if unusual {
			offers[i].known = false
		}
	}

	sent := 0
	for _, w := range watchers {
		if len(w.Sources) > 0 && !slices.Contains(w.Sources, sourceID) {
			continue // the member tracks this product at another store
		}
		// cheapestWhere finds the tracker's offer: its pinned listing, or the
		// cheapest of its variant, at the refreshed source or at all its stores.
		cheapestWhere := func(allStores bool, match func(offer) bool) (offer, bool) {
			var best offer
			found := false
			for _, o := range offers {
				switch {
				case w.OfferID != nil && o.id != *w.OfferID,
					w.OfferID == nil && w.Variant != "" && o.variant != w.Variant,
					!allStores && o.source != sourceID,
					allStores && len(w.Sources) > 0 && !slices.Contains(w.Sources, o.source):
					continue
				}
				if o.known && match(o) && (!found || o.cny < best.cny) {
					best, found = o, true
				}
			}
			return best, found
		}
		tracked, haveTracked := cheapestWhere(true, func(offer) bool { return true })

		channels := c.channels(w.FeishuWebhook, w.EmailAlerts)
		for _, rule := range Parse(w.Rules) {
			if len(channels) == 0 {
				break
			}
			var hit offer
			var ok bool
			var before, after *int64
			switch rule.Type {
			case RuleBelow:
				if rule.Below == nil {
					continue
				}
				limit, err := rates.ToCNYMinor(rule.Below.Minor, rule.Below.Currency)
				if err != nil {
					continue
				}
				hit, ok = cheapestWhere(false, func(o offer) bool { return o.cny < limit })
			case RuleDrop:
				if w.BaselinePriceCnyMinor == nil || rule.Percent <= 0 {
					continue
				}
				limit := *w.BaselinePriceCnyMinor * int64(100-rule.Percent) / 100
				hit, ok = cheapestWhere(false, func(o offer) bool { return o.cny <= limit })
			case RuleBackInStock:
				hit, ok = cheapestWhere(false, func(o offer) bool { return ch.BackInStock[o.id] })
			case RuleNewSource:
				if ch.FirstFromSource {
					hit, ok = cheapestWhere(false, func(offer) bool { return true })
				}
			case RuleChange:
				last := w.LastPriceCnyMinor
				if haveTracked && last != nil && *last > 0 && Moved(*last, tracked.cny, rule.Percent) {
					hit, ok = tracked, true
					before, after = last, &tracked.cny
				}
			}
			if !ok {
				continue
			}
			for _, channel := range channels {
				id, err := q.InsertAlert(ctx, store.InsertAlertParams{
					WatchItemID: w.ID, OfferID: hit.id, Rule: rule.Key(), Channel: channel,
					PreviousCnyMinor: before, CurrentCnyMinor: after,
				})
				if errors.Is(err, pgx.ErrNoRows) {
					continue // already alerted today (T5)
				}
				if err != nil {
					return sent, err
				}
				if err := c.Enqueue(ctx, id); err != nil {
					return sent, fmt.Errorf("enqueue alert %d: %w", id, err)
				}
				sent++
			}
		}
		if haveTracked && (w.LastPriceCnyMinor == nil || *w.LastPriceCnyMinor != tracked.cny) {
			if err := q.SetWatchLastPrice(ctx, store.SetWatchLastPriceParams{ID: w.ID, Price: &tracked.cny}); err != nil {
				return sent, err
			}
		}
	}
	return sent, nil
}

// Moved reports whether a price moved by at least percent, up or down.
func Moved(from, to int64, percent int) bool {
	diff := to - from
	if diff < 0 {
		diff = -diff
	}
	return diff*100 >= from*int64(percent)
}

func (c *Checker) channels(feishu string, email bool) []string {
	var out []string
	if feishu != "" {
		out = append(out, ChannelFeishu)
	}
	if email && c.EmailEnabled {
		out = append(out, ChannelEmail)
	}
	return out
}
