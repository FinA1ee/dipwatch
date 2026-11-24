package sources

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	failuresBeforeUnhealthy = 3
	unhealthyFor            = time.Hour
)

// Registry runs a source's strategies in order and remembers which ones keep failing.
// Health is in memory: a worker restart gives every strategy a fresh start, which is
// acceptable because the cooldown is only an hour.
type Registry struct {
	mu     sync.Mutex
	health map[healthKey]*health
	now    func() time.Time
}

type healthKey struct{ source, strategy string }

type health struct {
	failures       int
	unhealthyUntil time.Time
}

func NewRegistry() *Registry {
	return &Registry{health: map[healthKey]*health{}, now: time.Now}
}

// Search returns the offers of the first healthy strategy that finds any. A strategy
// that returns no offers without an error does not count as a failure, but the next
// one is still tried. It returns an error only when every strategy errored or was skipped.
func (r *Registry) Search(ctx context.Context, src Source, q Query) (Result, error) {
	var errs []error
	for _, s := range src.Strategies {
		key := healthKey{src.ID, s.Name()}
		if r.isUnhealthy(key) {
			errs = append(errs, fmt.Errorf("%s: skipped, unhealthy", s.Name()))
			continue
		}

		offers, err := s.Search(ctx, q)
		if err != nil {
			r.recordFailure(key)
			slog.WarnContext(ctx, "source strategy failed", "source", src.ID, "strategy", s.Name(), "err", err)
			errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
			continue
		}
		r.recordSuccess(key)
		if len(offers) > 0 {
			return Result{Strategy: s.Name(), Offers: offers}, nil
		}
	}
	if len(errs) == len(src.Strategies) && len(errs) > 0 {
		return Result{}, fmt.Errorf("source %s: %w", src.ID, errors.Join(errs...))
	}
	return Result{}, nil
}

func (r *Registry) isUnhealthy(key healthKey) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	h := r.health[key]
	return h != nil && r.now().Before(h.unhealthyUntil)
}

func (r *Registry) recordFailure(key healthKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h := r.health[key]
	if h == nil {
		h = &health{}
		r.health[key] = h
	}
	h.failures++
	if h.failures >= failuresBeforeUnhealthy {
		h.unhealthyUntil = r.now().Add(unhealthyFor)
		h.failures = 0
	}
}

func (r *Registry) recordSuccess(key healthKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.health, key)
}
