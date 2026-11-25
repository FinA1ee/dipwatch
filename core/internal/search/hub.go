// Package search answers search queries (spec S1, S2, S5): it resolves the query
// to a product, returns ranked offers, starts refreshes, and streams each source's
// results as the worker finishes them.
package search

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/jobs"
)

// Hub holds one Postgres LISTEN connection and fans worker notifications out to
// the open streams of each product.
type Hub struct {
	DB *pgxpool.Pool

	mu   sync.Mutex
	subs map[int64]map[chan jobs.SourceDone]struct{}
}

// Subscribe returns a channel of notifications for one product and a function
// that ends the subscription.
func (h *Hub) Subscribe(productID int64) (<-chan jobs.SourceDone, func()) {
	ch := make(chan jobs.SourceDone, 16)
	h.mu.Lock()
	if h.subs == nil {
		h.subs = map[int64]map[chan jobs.SourceDone]struct{}{}
	}
	if h.subs[productID] == nil {
		h.subs[productID] = map[chan jobs.SourceDone]struct{}{}
	}
	h.subs[productID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[productID], ch)
		if len(h.subs[productID]) == 0 {
			delete(h.subs, productID)
		}
		h.mu.Unlock()
	}
}

func (h *Hub) publish(d jobs.SourceDone) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[d.ProductID] {
		select {
		case ch <- d:
		default: // a slow stream catches up from the database on its next poll
		}
	}
}

// Run listens until ctx ends, reconnecting after errors. Streams also poll the
// database, so notifications lost while reconnecting only delay them.
func (h *Hub) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := h.listen(ctx); err != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "search hub: listen failed, retrying", "err", err)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
			}
		}
	}
}

func (h *Hub) listen(ctx context.Context) error {
	conn, err := h.DB.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+jobs.OffersChannel); err != nil {
		return err
	}
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var d jobs.SourceDone
		if err := json.Unmarshal([]byte(n.Payload), &d); err != nil {
			slog.WarnContext(ctx, "search hub: bad payload", "payload", n.Payload, "err", err)
			continue
		}
		h.publish(d)
	}
}
