package search

import (
	"context"
	"slices"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/store"
)

const (
	// StreamTimeout ends a stream whose sources never all report, e.g. after a
	// worker crash; the next search starts a new refresh.
	StreamTimeout = 5 * time.Minute
	// pollEvery re-reads the refresh state in case a notification was lost, and
	// keeps proxies from closing an idle connection.
	pollEvery = 10 * time.Second
)

// OffersEvent replaces the client's offer list. Every event carries the whole
// ranked list, so a client that connects late or misses an event is still right.
type OffersEvent struct {
	Offers  []Offer  `json:"offers" doc:"Cheapest first, compared in CNY"`
	Pending []string `json:"pending" doc:"Ids of sources still searching"`
}

// DoneEvent ends the stream.
type DoneEvent struct {
	TimedOut bool `json:"timed_out" doc:"true when some sources did not finish in time"`
}

// Events is the stream's event names, for the API contract.
var Events = map[string]any{"offers": OffersEvent{}, "done": DoneEvent{}}

// Emit sends one event; it fails when the client has gone.
type Emit func(event any) error

// Stream sends the product's offers now and again each time a source finishes,
// then a DoneEvent once no source is running (S2).
func (s *Service) Stream(ctx context.Context, productID int64, currency string, emit Emit) error {
	updates, stop := s.Hub.Subscribe(productID)
	defer stop()
	q := store.New(s.DB)
	// Fail before streaming when the product does not exist.
	if _, err := q.GetProduct(ctx, productID); err != nil {
		return err
	}

	deadline := time.After(StreamTimeout)
	poll := time.NewTicker(pollEvery)
	defer poll.Stop()
	var last []string // pending sources at the previous event
	first := true
	for {
		product, err := q.GetProduct(ctx, productID)
		if err != nil {
			return err
		}
		pending := product.RefreshingSources
		if pending == nil {
			pending = []string{}
		}
		if first || !slices.Equal(pending, last) {
			offers, err := s.offers(ctx, q, productID, nil, currency)
			if err != nil {
				return err
			}
			if err := emit(OffersEvent{Offers: offers, Pending: pending}); err != nil {
				return nil // client gone
			}
			first, last = false, pending
		}
		if len(pending) == 0 {
			_ = emit(DoneEvent{})
			return nil
		}

		select {
		case <-updates:
		case <-poll.C:
		case <-deadline:
			_ = emit(DoneEvent{TimedOut: true})
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}
