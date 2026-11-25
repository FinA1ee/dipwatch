// Package spend is the spend breaker (spec C4): today's LLM spend against the
// owner's daily budget decides what may run.
package spend

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/limits"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// Level is what today's spend allows.
type Level int

const (
	Normal      Level = iota // under 80%: everything runs
	TrackedOnly              // 80% to 100%: agent runs only for tracked products
	StoredOnly               // 100%: no LLM calls; stored data only until the next day
)

func (l Level) String() string {
	return [...]string{"normal", "tracked_only", "stored_only"}[l]
}

// Meter reports what today's spend allows. *Breaker is the real one.
type Meter interface {
	Level(ctx context.Context) Level
}

// LevelOf is m.Level, treating a nil meter as no budget.
func LevelOf(ctx context.Context, m Meter) Level {
	if m == nil {
		return Normal
	}
	return m.Level(ctx)
}

// ErrPaused means today's budget is used up and the step would cost money.
var ErrPaused = errors.New("today's budget is used up")

// China is the calendar the daily budget follows.
var China = time.FixedZone("CST", 8*60*60)

// cacheFor keeps the spend query off the hot path; spend moves slowly.
const cacheFor = 15 * time.Second

// Status is today's spend for the admin page.
type Status struct {
	TodayCNY  float64 `json:"today_cny"`
	BudgetCNY float64 `json:"budget_cny"`
	Level     string  `json:"level" enum:"normal,tracked_only,stored_only"`
}

// Breaker reads today's spend and the budget.
type Breaker struct {
	DB     *pgxpool.Pool
	Limits *limits.Store    // nil: the defaults
	Now    func() time.Time // nil: time.Now

	mu      sync.Mutex
	micros  int64
	day     string
	fetched time.Time
}

// Level is what may run now. On a database error it allows everything: a
// broken spend query must not take the site down, and agent runs keep their own
// per-run budget (C3).
func (b *Breaker) Level(ctx context.Context) Level {
	if b == nil {
		return Normal
	}
	st, err := b.Status(ctx)
	if err != nil {
		return Normal
	}
	return levelOf(st.TodayCNY, st.BudgetCNY)
}

func levelOf(today, budget float64) Level {
	switch {
	case budget <= 0 || today >= budget:
		return StoredOnly
	case today >= 0.8*budget:
		return TrackedOnly
	}
	return Normal
}

func (b *Breaker) Status(ctx context.Context) (Status, error) {
	now := time.Now
	if b.Now != nil {
		now = b.Now
	}
	budget := limits.Defaults.DailyBudgetCNY
	if b.Limits != nil {
		budget = b.Limits.Get(ctx).DailyBudgetCNY
	}
	today := now().In(China)
	day := today.Format(time.DateOnly)

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.day != day || time.Since(b.fetched) >= cacheFor {
		midnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, China)
		micros, err := store.New(b.DB).SpendSince(ctx, pgtype.Timestamptz{Time: midnight, Valid: true})
		if err != nil {
			return Status{}, err
		}
		b.micros, b.day, b.fetched = micros, day, time.Now()
	}
	spent := float64(b.micros) / 1e6
	return Status{TodayCNY: spent, BudgetCNY: budget, Level: levelOf(spent, budget).String()}, nil
}
