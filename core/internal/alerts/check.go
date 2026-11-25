package alerts

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/FinA1ee/dipwatch/core/internal/fx"
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
	id    int64
	cny   int64
	known bool
}

// Check runs every tracker's rules against one source's offers for a product
// (T1, T4). Each rule fires for the cheapest matching offer, at most once per
// offer, rule and channel per day (T5, enforced by the alerts table).
func (c *Checker) Check(ctx context.Context, q *store.Queries, productID int64, sourceID string, ch Changes) (int, error) {
	watchers, err := q.ListProductWatchers(ctx, productID)
	if err != nil || len(watchers) == 0 {
		return 0, err
	}
	rows, err := q.ListAlertOffers(ctx, store.ListAlertOffersParams{ProductID: productID, SourceID: sourceID})
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	rates, err := fx.Latest(ctx, q)
	if err != nil {
		rates = fx.Rates{} // CNY offers can still be compared
	}
	offers := make([]offer, 0, len(rows))
	for _, r := range rows {
		cny, err := rates.ToCNYMinor(r.PriceMinor, r.Currency)
		offers = append(offers, offer{id: r.ID, cny: cny, known: err == nil})
	}
	cheapestWhere := func(match func(offer) bool) (offer, bool) {
		var best offer
		found := false
		for _, o := range offers {
			if o.known && match(o) && (!found || o.cny < best.cny) {
				best, found = o, true
			}
		}
		return best, found
	}

	sent := 0
	for _, w := range watchers {
		channels := c.channels(w.FeishuWebhook, w.EmailAlerts)
		if len(channels) == 0 {
			continue
		}
		for _, rule := range Parse(w.Rules) {
			var hit offer
			var ok bool
			switch rule.Type {
			case RuleBelow:
				if rule.Below == nil {
					continue
				}
				limit, err := rates.ToCNYMinor(rule.Below.Minor, rule.Below.Currency)
				if err != nil {
					continue
				}
				hit, ok = cheapestWhere(func(o offer) bool { return o.cny < limit })
			case RuleDrop:
				if w.BaselinePriceCnyMinor == nil || rule.Percent <= 0 {
					continue
				}
				limit := *w.BaselinePriceCnyMinor * int64(100-rule.Percent) / 100
				hit, ok = cheapestWhere(func(o offer) bool { return o.cny <= limit })
			case RuleBackInStock:
				hit, ok = cheapestWhere(func(o offer) bool { return ch.BackInStock[o.id] })
			case RuleNewSource:
				if ch.FirstFromSource {
					hit, ok = cheapestWhere(func(offer) bool { return true })
				}
			}
			if !ok {
				continue
			}
			for _, channel := range channels {
				id, err := q.InsertAlert(ctx, store.InsertAlertParams{WatchItemID: w.ID, OfferID: hit.id, Rule: rule.Key(), Channel: channel})
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
	}
	return sent, nil
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
