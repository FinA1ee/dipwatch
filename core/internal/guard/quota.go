package guard

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// Daily limits on searches that cost money (spec C1). Cached answers are free.
const (
	VisitorDailySearches = 10
	MemberDailySearches  = 30
)

// ErrQuotaExceeded means the viewer used today's costly searches.
var ErrQuotaExceeded = errors.New("daily search limit reached")

// Viewer is who is asking, as the gateway reported it.
type Viewer struct {
	UserID string // empty for visitors
	Email  string
	Admin  bool
	IP     string
}

// quotaSubject and limit: members are counted by account, visitors by IP.
func (v Viewer) quotaSubject() (string, int) {
	if v.UserID != "" {
		return "user:" + v.UserID, MemberDailySearches
	}
	ip := v.IP
	if ip == "" {
		ip = "unknown" // only reachable without the gateway in front
	}
	return "ip:" + ip, VisitorDailySearches
}

// Quotas counts costly searches in app.search_quota.
type Quotas struct {
	DB *pgxpool.Pool
}

// Charge counts one costly search for the viewer, or returns ErrQuotaExceeded.
func (q *Quotas) Charge(ctx context.Context, v Viewer) error {
	subject, limit := v.quotaSubject()
	_, err := store.New(q.DB).ChargeQuota(ctx, store.ChargeQuotaParams{Subject: subject, QuotaLimit: int32(limit)}) //nolint:gosec // small constant
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrQuotaExceeded
	}
	return err
}

// Charger charges a search at most once, just before its first costly step, so
// a search that needs both the intent call and a refresh counts once.
type Charger struct {
	Quotas *Quotas // nil: no limits (tools and tests)
	Viewer Viewer
	done   bool
}

func (c *Charger) Charge(ctx context.Context) error {
	if c == nil || c.Quotas == nil || c.done {
		return nil
	}
	if err := c.Quotas.Charge(ctx, c.Viewer); err != nil {
		return err
	}
	c.done = true
	return nil
}
