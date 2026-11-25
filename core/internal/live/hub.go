package live

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/db"
)

// KeepRuns is how many recent runs the hub remembers for a page opened later.
const KeepRuns = 20

// Run is a run as seen so far: its start, the steps that arrived, and its end.
type Run struct {
	Start Event   `json:"start"`
	Steps []Event `json:"steps"`
	End   *Event  `json:"end,omitempty" doc:"Missing while the run is in progress"`
}

// Hub collects agent events from Postgres and fans them out to admin streams.
// It lives in memory: a core-api restart forgets runs in progress, whose full
// trace is still recorded in agent_runs when they finish.
type Hub struct {
	DB *pgxpool.Pool

	mu    sync.Mutex
	runs  []*Run // oldest first
	index map[string]*Run
	subs  map[chan Event]struct{}
}

// Recent returns copies of the remembered runs, newest first.
func (h *Hub) Recent() []Run {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Run, 0, len(h.runs))
	for i := len(h.runs) - 1; i >= 0; i-- {
		r := *h.runs[i]
		r.Steps = append([]Event(nil), r.Steps...)
		if r.Steps == nil {
			r.Steps = []Event{}
		}
		out = append(out, r)
	}
	return out
}

// Subscribe returns a channel of new events and a function ending the subscription.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	if h.subs == nil {
		h.subs = map[chan Event]struct{}{}
	}
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// Add records an event and sends it to every subscriber.
func (h *Hub) Add(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.index == nil {
		h.index = map[string]*Run{}
	}
	r := h.index[e.Run]
	switch {
	case e.Kind == KindStart:
		r = &Run{Start: e, Steps: []Event{}}
		h.index[e.Run] = r
		h.runs = append(h.runs, r)
		if len(h.runs) > KeepRuns {
			delete(h.index, h.runs[0].Start.Run)
			h.runs = h.runs[1:]
		}
	case r == nil:
		// Its start was missed (core-api started mid-run): still stream it, not stored.
	case e.Kind == KindStep:
		r.Steps = append(r.Steps, e)
	case e.Kind == KindEnd:
		end := e
		r.End = &end
	}
	for ch := range h.subs {
		select {
		case ch <- e:
		default: // a slow page misses this event; reloading shows the stored state
		}
	}
}

// Run listens for agent events until ctx ends.
func (h *Hub) Run(ctx context.Context) {
	db.Listen(ctx, h.DB, Channel, func(payload string) {
		var e Event
		if err := json.Unmarshal([]byte(payload), &e); err != nil {
			slog.WarnContext(ctx, "live hub: bad payload", "err", err)
			return
		}
		h.Add(e)
	})
}
