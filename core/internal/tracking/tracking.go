// Package tracking manages what members track (spec T1, T2, C2) and where
// their alerts go (T6).
package tracking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/alerts"
	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/guard"
	"github.com/FinA1ee/dipwatch/core/internal/limits"
	"github.com/FinA1ee/dipwatch/core/internal/matcher"
	"github.com/FinA1ee/dipwatch/core/internal/notify"
	"github.com/FinA1ee/dipwatch/core/internal/pricetrust"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// MaxNoteRune is the longest note (T2). The tracked-products limit is the owner's
// live setting (limits.TrackedPerMember).
const MaxNoteRune = 500

// Errors the API maps to 4xx responses.
var (
	ErrNotMember   = errors.New("sign in to track products")
	ErrNoteTooLong = fmt.Errorf("notes can be up to %d characters", MaxNoteRune)
	ErrNotTracked  = errors.New("you are not tracking this product")
	ErrNoProduct   = errors.New("no such product")
	ErrNoWebhook   = errors.New("add a Feishu webhook first")
	ErrStores      = errors.New("pick the store to track this product at")
	ErrListing     = errors.New("that listing is not an offer for this product")
)

// MaxCheckHours is the longest re-check interval a member may choose.
const MaxCheckHours = 7 * 24

// CheckEveryError means the chosen re-check interval is outside what is allowed.
type CheckEveryError struct{ Min int }

func (e *CheckEveryError) Error() string {
	return fmt.Sprintf("re-check every %d to %d hours, or leave it automatic", e.Min, MaxCheckHours)
}

// LimitError means the member reached the tracked-products limit.
type LimitError struct{ Limit int }

func (e *LimitError) Error() string {
	if e.Limit == 1 {
		return "you can track 1 product for now: stop tracking the other one first"
	}
	return fmt.Sprintf("you can track up to %d products", e.Limit)
}

// Item is one tracked product.
type Item struct {
	ProductID       int64         `json:"product_id"`
	ProductName     string        `json:"product_name"`
	Note            string        `json:"note"`
	Rules           []alerts.Rule `json:"rules"`
	Stores          []string      `json:"stores" doc:"Stores refreshed and alerted on; empty: every store"`
	Variant         string        `json:"variant" doc:"Only listings of this variant count, e.g. 24GB; empty: any variant"`
	Baseline        *alerts.Money `json:"baseline,omitempty" doc:"Price when tracking started (of the listing, if one is pinned), in CNY"`
	Listing         *Listing      `json:"listing,omitempty" doc:"The listing this tracker follows; absent: the product's cheapest offer"`
	History         []DayPrice    `json:"history" doc:"The tracked price per day since tracking started (at most a year), oldest first: the listing's, or the lowest of the variant at the tracker's stores"`
	CheckEveryHours int           `json:"check_every_hours" doc:"Chosen re-check interval in hours; 0: automatic (6 hours, 2 to 24 as prices move)"`
	LastChecked     time.Time     `json:"last_checked,omitzero" doc:"When the product was last refreshed"`
	NextCheck       time.Time     `json:"next_check,omitzero" doc:"When the next scheduled refresh is due, at the latest"`
	TrackedAt       time.Time     `json:"tracked_at"`
}

// DayPrice is the tracked price on one UTC day.
type DayPrice struct {
	Day   string       `json:"day" format:"date"`
	Price alerts.Money `json:"price" doc:"In CNY"`
}

// Listing is a pinned offer, as last seen.
type Listing struct {
	OfferID int64        `json:"offer_id"`
	Title   string       `json:"title"`
	Store   string       `json:"store" doc:"Source id"`
	URL     string       `json:"url"`
	Price   alerts.Money `json:"price" doc:"Price at the store, in its currency"`
	Variant string       `json:"variant" doc:"The listing's variant, e.g. 24GB; empty when its title does not say"`
	Gone    bool         `json:"gone" doc:"The store's last refresh no longer listed it"`
}

// Settings are a member's alert channels.
type Settings struct {
	FeishuWebhook  string `json:"feishu_webhook" doc:"Empty: no Feishu alerts"`
	EmailAlerts    bool   `json:"email_alerts"`
	EmailAvailable bool   `json:"email_available" doc:"false while the site has no SMTP: email alerts are not sent yet"`
	Email          string `json:"email"`
}

type Service struct {
	DB             *pgxpool.Pool
	Feishu         notify.Feishu
	EmailAvailable bool
	Limits         *limits.Store // nil: the defaults
	// CheckNow queues a rule check on stored offers after a save, so a rule
	// that already holds alerts at once (nil: only after the next refresh).
	CheckNow func(ctx context.Context, productID int64, sources []string) error
}

func (s *Service) limits(ctx context.Context) limits.Limits {
	if s.Limits == nil {
		return limits.Defaults
	}
	return s.Limits.Get(ctx)
}

// member makes sure core has a row for the signed-in member (Better Auth owns
// the account; app.users holds settings and foreign keys).
func (s *Service) member(ctx context.Context, v guard.Viewer) (*store.Queries, error) {
	if v.UserID == "" {
		return nil, ErrNotMember
	}
	q := store.New(s.DB)
	if err := q.UpsertUser(ctx, store.UpsertUserParams{ID: v.UserID, Email: v.Email}); err != nil {
		return nil, err
	}
	return q, nil
}

func (s *Service) List(ctx context.Context, v guard.Viewer) ([]Item, error) {
	q, err := s.member(ctx, v)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListWatchItems(ctx, v.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]Item, len(rows))
	for i, r := range rows {
		out[i] = item(r.ProductID, r.CanonicalName, r.Note, r.Rules, r.BaselinePriceCnyMinor, r.CreatedAt.Time, r.Sources)
		out[i].Listing = listing(r)
		out[i].Variant = r.Variant
		out[i].CheckEveryHours = hours(r.CheckEvery)
		if r.ProductRefreshedAt.Valid {
			out[i].LastChecked = r.ProductRefreshedAt.Time
			every := r.ProductRefreshInterval
			if c := hours(r.CheckEvery); c > 0 && time.Duration(c)*time.Hour < duration(every) {
				out[i].NextCheck = r.ProductRefreshedAt.Time.Add(time.Duration(c) * time.Hour)
			} else {
				out[i].NextCheck = r.ProductRefreshedAt.Time.Add(duration(every))
			}
		}
	}
	if len(out) == 0 {
		return out, nil
	}
	rates, err := fx.Latest(ctx, q)
	if err != nil {
		return out, nil // no rates yet: no history to compare
	}
	for i, r := range rows {
		if out[i].History, err = history(ctx, q, rates, r); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Save starts tracking a product, or updates its note and rules. A tracker
// follows the cheapest offer of a variant ("" any) at the given stores, or,
// with offerID, one listing at its store. The stores and the baseline price are
// set when tracking starts and again when the listing or variant changes.
func (s *Service) Save(ctx context.Context, v guard.Viewer, productID int64, note string, rules []alerts.Rule, stores []string, offerID *int64, variant string, checkEveryHours int) (Item, error) {
	q, err := s.member(ctx, v)
	if err != nil {
		return Item{}, err
	}
	if utf8.RuneCountInString(note) > MaxNoteRune {
		return Item{}, ErrNoteTooLong
	}
	if err := alerts.Validate(rules); err != nil {
		return Item{}, err
	}
	product, err := q.GetProduct(ctx, productID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNoProduct
	}
	if err != nil {
		return Item{}, err
	}
	existing, err := q.GetWatchItem(ctx, store.GetWatchItemParams{UserID: v.UserID, ProductID: productID})
	isNew := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !isNew {
		return Item{}, err
	}
	l := s.limits(ctx)
	if !guard.Invited(l, v) {
		return Item{}, guard.ErrInviteOnly
	}
	if checkEveryHours != 0 && (checkEveryHours < max(l.MinCheckHours, 1) || checkEveryHours > MaxCheckHours) {
		return Item{}, &CheckEveryError{Min: max(l.MinCheckHours, 1)}
	}
	if isNew {
		count, err := q.CountWatchItems(ctx, v.UserID)
		if err != nil {
			return Item{}, err
		}
		if int(count) >= l.TrackedPerMember {
			return Item{}, &LimitError{Limit: l.TrackedPerMember}
		}
	}

	variant = matcher.NormalizeVariant(variant)
	baseline, sources := existing.BaselinePriceCnyMinor, existing.Sources
	switch {
	case offerID != nil && (isNew || !sameID(existing.OfferID, offerID)):
		o, err := q.GetProductOffer(ctx, store.GetProductOfferParams{ID: *offerID, ProductID: productID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && o.IsProduct != nil && !*o.IsProduct) {
			return Item{}, ErrListing
		}
		if err != nil {
			return Item{}, err
		}
		sources = []string{o.SourceID}
		variant = "" // the listing decides
		baseline = nil
		if rates, err := fx.Latest(ctx, q); err == nil {
			if cny, err := rates.ToCNYMinor(o.PriceMinor, o.Currency); err == nil {
				baseline = &cny
			}
		}
	case offerID != nil:
		variant = "" // same listing as before
	case isNew || existing.OfferID != nil || existing.Variant != variant:
		if isNew {
			if len(stores) == 0 || len(stores) > l.SourcesPerSearch {
				return Item{}, ErrStores
			}
			sources = stores
		}
		if baseline, err = lowestCNY(ctx, q, productID, sources, variant); err != nil {
			return Item{}, err
		}
	}
	if sources == nil {
		sources = []string{}
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return Item{}, err
	}
	w, err := q.SaveWatchItem(ctx, store.SaveWatchItemParams{
		UserID: v.UserID, ProductID: productID, Note: note, Rules: raw,
		BaselinePriceCnyMinor: baseline, Sources: sources, OfferID: offerID, Variant: variant,
		CheckEveryHours: int32(checkEveryHours), //nolint:gosec // checked above: at most a week
	})
	if err != nil {
		return Item{}, err
	}
	if s.CheckNow != nil {
		if err := s.CheckNow(ctx, productID, w.Sources); err != nil {
			slog.WarnContext(ctx, "tracking: queue rule check", "product", productID, "err", err)
		}
	}
	for _, it := range must(s.List(ctx, v)) {
		if it.ProductID == productID {
			return it, nil
		}
	}
	return item(w.ProductID, product.CanonicalName, w.Note, w.Rules, w.BaselinePriceCnyMinor, w.CreatedAt.Time, w.Sources), nil
}

// history is the tracker's price per day: its listing's, or the lowest of its
// variant at its stores. Unconfirmed prices are left out.
func history(ctx context.Context, q *store.Queries, rates fx.Rates, r store.ListWatchItemsRow) ([]DayPrice, error) {
	// From the day tracking started: prices before it are not what the member follows.
	start := r.CreatedAt.Time.UTC().Truncate(24 * time.Hour)
	if oldest := time.Now().UTC().AddDate(-1, 0, 0).Truncate(24 * time.Hour); start.Before(oldest) {
		start = oldest
	}
	since := pgtype.Timestamptz{Time: start, Valid: true}
	var points []fx.Point
	if r.OfferID != nil {
		rows, err := q.ListOfferPricePoints(ctx, store.ListOfferPricePointsParams{OfferID: *r.OfferID, ProductID: r.ProductID, Since: since})
		if err != nil {
			return nil, err
		}
		for _, p := range rows {
			points = append(points, fx.Point{At: p.ObservedAt.Time, Minor: p.PriceMinor, Currency: p.Currency})
		}
	} else {
		sources := r.Sources
		if sources == nil {
			sources = []string{}
		}
		rows, err := q.ListProductPricePoints(ctx, store.ListProductPricePointsParams{ProductID: r.ProductID, Since: since, Variant: r.Variant, SourceIds: sources})
		if err != nil {
			return nil, err
		}
		for _, p := range rows {
			points = append(points, fx.Point{At: p.ObservedAt.Time, Minor: p.PriceMinor, Currency: p.Currency})
		}
	}
	lows := rates.DailyLows(points)
	// The baseline is the tracked price when tracking started; it opens the line
	// when no refresh has recorded a price on that day yet.
	if b := r.BaselinePriceCnyMinor; b != nil && (len(lows) == 0 || lows[0].Day != start.Format(time.DateOnly)) {
		lows = append([]fx.DayLow{{Day: start.Format(time.DateOnly), CNY: *b}}, lows...)
	}
	lows = fx.FillDays(lows, time.Now())
	out := make([]DayPrice, len(lows))
	for i, d := range lows {
		out[i] = DayPrice{Day: d.Day, Price: alerts.Money{Minor: d.CNY, Currency: "CNY"}}
	}
	return out, nil
}

// hours reads a Postgres interval as whole hours; NULL or invalid is 0.
func hours(i pgtype.Interval) int {
	if !i.Valid {
		return 0
	}
	return int(duration(i) / time.Hour)
}

func duration(i pgtype.Interval) time.Duration {
	if !i.Valid {
		return 6 * time.Hour
	}
	return time.Duration(i.Microseconds)*time.Microsecond + time.Duration(i.Days)*24*time.Hour
}

func sameID(a, b *int64) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

// must drops a List error: Save already succeeded, and the fallback item
// without listing details is still correct.
func must(items []Item, err error) []Item {
	if err != nil {
		return nil
	}
	return items
}

// goneAfter is how much later than a listing's last sighting its store's
// refresh must be to count the listing as gone (saving offers and marking the
// refresh happen moments apart).
const goneAfter = 5 * time.Minute

func listing(r store.ListWatchItemsRow) *Listing {
	if r.OfferID == nil || r.OfferTitle == nil {
		return nil
	}
	gone := r.OfferSourceRefreshedAt.Valid && r.OfferLastSeenAt.Valid &&
		r.OfferSourceRefreshedAt.Time.Sub(r.OfferLastSeenAt.Time) > goneAfter
	return &Listing{
		OfferID: *r.OfferID, Title: *r.OfferTitle, Store: deref(r.OfferSourceID), URL: deref(r.OfferUrl),
		Price: alerts.Money{Minor: derefInt(r.OfferPriceMinor), Currency: deref(r.OfferCurrency)}, Gone: gone,
		Variant: deref(r.OfferVariant),
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}

func (s *Service) Delete(ctx context.Context, v guard.Viewer, productID int64) error {
	q, err := s.member(ctx, v)
	if err != nil {
		return err
	}
	n, err := q.DeleteWatchItem(ctx, store.DeleteWatchItemParams{UserID: v.UserID, ProductID: productID})
	if err == nil && n == 0 {
		return ErrNotTracked
	}
	return err
}

func (s *Service) Settings(ctx context.Context, v guard.Viewer) (Settings, error) {
	q, err := s.member(ctx, v)
	if err != nil {
		return Settings{}, err
	}
	u, err := q.GetUser(ctx, v.UserID)
	if err != nil {
		return Settings{}, err
	}
	return Settings{FeishuWebhook: u.FeishuWebhook, EmailAlerts: u.EmailAlerts, EmailAvailable: s.EmailAvailable, Email: u.Email}, nil
}

// SaveSettings stores the channels; a webhook must pass the host check (T6).
func (s *Service) SaveSettings(ctx context.Context, v guard.Viewer, feishu string, email bool) (Settings, error) {
	q, err := s.member(ctx, v)
	if err != nil {
		return Settings{}, err
	}
	if feishu != "" {
		if err := notify.CheckWebhook(feishu); err != nil {
			return Settings{}, err
		}
	}
	if err := q.SetUserChannels(ctx, store.SetUserChannelsParams{ID: v.UserID, FeishuWebhook: feishu, EmailAlerts: email}); err != nil {
		return Settings{}, err
	}
	return s.Settings(ctx, v)
}

// TestFeishu sends a test message to the member's saved webhook.
func (s *Service) TestFeishu(ctx context.Context, v guard.Viewer) error {
	set, err := s.Settings(ctx, v)
	if err != nil {
		return err
	}
	if set.FeishuWebhook == "" {
		return ErrNoWebhook
	}
	return s.Feishu.Send(ctx, set.FeishuWebhook, "dipwatch: test message. Price alerts for your tracked products will arrive here.")
}

// lowestCNY is the cheapest comparable offer in CNY at the stores, of the
// variant ("" any): the baseline of drop rules. Unconfirmed and unusual prices
// do not count (pricetrust).
func lowestCNY(ctx context.Context, q *store.Queries, productID int64, stores []string, variant string) (*int64, error) {
	rows, err := q.ListSearchOffers(ctx, store.ListSearchOffersParams{ProductID: productID, SourceIds: stores})
	if err != nil {
		return nil, err
	}
	rates, err := fx.Latest(ctx, q)
	if err != nil {
		rates = fx.Rates{}
	}
	prices := make([]pricetrust.Price, len(rows))
	for i, r := range rows {
		cny, err := rates.ToCNYMinor(r.PriceMinor, r.Currency)
		prices[i] = pricetrust.Price{CNY: cny, Known: err == nil && !r.PriceUnconfirmed, Variant: r.Variant}
	}
	unusual := pricetrust.Unusual(prices)
	var low *int64
	for i, p := range prices {
		if !p.Known || unusual[i] || (variant != "" && p.Variant != variant) {
			continue
		}
		if low == nil || p.CNY < *low {
			v := p.CNY
			low = &v
		}
	}
	return low, nil
}

func item(productID int64, name, note string, raw []byte, baseline *int64, at time.Time, stores []string) Item {
	if stores == nil {
		stores = []string{}
	}
	it := Item{ProductID: productID, ProductName: name, Note: note, Rules: alerts.Parse(raw), TrackedAt: at, Stores: stores, History: []DayPrice{}}
	if it.Rules == nil {
		it.Rules = []alerts.Rule{}
	}
	if baseline != nil {
		it.Baseline = &alerts.Money{Minor: *baseline, Currency: "CNY"}
	}
	return it
}
