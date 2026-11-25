// Package tracking manages what members track (spec T1, T2, C2) and where
// their alerts go (T6).
package tracking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/alerts"
	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/guard"
	"github.com/FinA1ee/dipwatch/core/internal/notify"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// Limits from the spec.
const (
	MaxTracked  = 20  // C2
	MaxNoteRune = 500 // T2
)

// Errors the API maps to 4xx responses.
var (
	ErrNotMember   = errors.New("sign in to track products")
	ErrTooMany     = fmt.Errorf("you can track up to %d products", MaxTracked)
	ErrNoteTooLong = fmt.Errorf("notes can be up to %d characters", MaxNoteRune)
	ErrNotTracked  = errors.New("you are not tracking this product")
	ErrNoProduct   = errors.New("no such product")
	ErrNoWebhook   = errors.New("add a Feishu webhook first")
)

// Item is one tracked product.
type Item struct {
	ProductID   int64         `json:"product_id"`
	ProductName string        `json:"product_name"`
	Note        string        `json:"note"`
	Rules       []alerts.Rule `json:"rules"`
	Baseline    *alerts.Money `json:"baseline,omitempty" doc:"Lowest price when tracking started, in CNY"`
	TrackedAt   time.Time     `json:"tracked_at"`
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
		out[i] = item(r.ProductID, r.CanonicalName, r.Note, r.Rules, r.BaselinePriceCnyMinor, r.CreatedAt.Time)
	}
	return out, nil
}

// Save starts tracking a product or updates its note and rules.
func (s *Service) Save(ctx context.Context, v guard.Viewer, productID int64, note string, rules []alerts.Rule) (Item, error) {
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
	_, err = q.GetWatchItem(ctx, store.GetWatchItemParams{UserID: v.UserID, ProductID: productID})
	isNew := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !isNew {
		return Item{}, err
	}
	var baseline *int64
	if isNew {
		count, err := q.CountWatchItems(ctx, v.UserID)
		if err != nil {
			return Item{}, err
		}
		if count >= MaxTracked {
			return Item{}, ErrTooMany
		}
		if baseline, err = lowestCNY(ctx, q, productID); err != nil {
			return Item{}, err
		}
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return Item{}, err
	}
	w, err := q.SaveWatchItem(ctx, store.SaveWatchItemParams{
		UserID: v.UserID, ProductID: productID, Note: note, Rules: raw, BaselinePriceCnyMinor: baseline,
	})
	if err != nil {
		return Item{}, err
	}
	return item(w.ProductID, product.CanonicalName, w.Note, w.Rules, w.BaselinePriceCnyMinor, w.CreatedAt.Time), nil
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
	return s.Feishu.Send(ctx, set.FeishuWebhook, "Dipwatch: test message. Price alerts for your tracked products will arrive here.")
}

// lowestCNY is the cheapest current offer in CNY, the baseline of drop rules.
func lowestCNY(ctx context.Context, q *store.Queries, productID int64) (*int64, error) {
	rows, err := q.ListSearchOffers(ctx, store.ListSearchOffersParams{ProductID: productID})
	if err != nil {
		return nil, err
	}
	rates, err := fx.Latest(ctx, q)
	if err != nil {
		rates = fx.Rates{}
	}
	var low *int64
	for _, r := range rows {
		if cny, err := rates.ToCNYMinor(r.PriceMinor, r.Currency); err == nil && (low == nil || cny < *low) {
			v := cny
			low = &v
		}
	}
	return low, nil
}

func item(productID int64, name, note string, raw []byte, baseline *int64, at time.Time) Item {
	it := Item{ProductID: productID, ProductName: name, Note: note, Rules: alerts.Parse(raw), TrackedAt: at}
	if it.Rules == nil {
		it.Rules = []alerts.Rule{}
	}
	if baseline != nil {
		it.Baseline = &alerts.Money{Minor: *baseline, Currency: "CNY"}
	}
	return it
}
