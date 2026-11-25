package guard

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/limits"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// ErrQuotaExceeded means the viewer used today's costly searches.
var ErrQuotaExceeded = errors.New("daily search limit reached")

// ErrInviteOnly means the site is invite-only and the viewer is not invited.
var ErrInviteOnly = errors.New("Dipwatch is invite-only for now: ask the owner to add your sign-in email")

// Invited reports whether the viewer may use costly features under the invite list.
func Invited(l limits.Limits, v Viewer) bool { return v.Admin || l.Allows(v.Email) }

// QuotaError is ErrQuotaExceeded with the limit that applied.
type QuotaError struct {
	Limit  int
	Member bool
}

func (e *QuotaError) Error() string        { return ErrQuotaExceeded.Error() }
func (e *QuotaError) Is(target error) bool { return target == ErrQuotaExceeded }

// Viewer is who is asking, as the gateway reported it.
type Viewer struct {
	UserID string // empty for visitors
	Email  string
	Admin  bool
	IP     string
}

// quotaSubject and limit: members are counted by account, visitors by IP.
// Daily limits on searches that cost money (spec C1) come from the owner's
// live limits; cached answers are free.
func (v Viewer) quotaSubject(l limits.Limits) (string, int) {
	if v.UserID != "" {
		return "user:" + v.UserID, l.MemberDailySearches
	}
	ip := v.IP
	if ip == "" {
		ip = "unknown" // only reachable without the gateway in front
	}
	return "ip:" + ip, l.VisitorDailySearches
}

// Quotas counts costly searches in app.search_quota.
type Quotas struct {
	DB     *pgxpool.Pool
	Limits *limits.Store // nil: the defaults
}

// DailyLimit is the viewer's number of costly searches per day.
func (q *Quotas) DailyLimit(ctx context.Context, v Viewer) int {
	_, limit := v.quotaSubject(q.current(ctx))
	return limit
}

func (q *Quotas) current(ctx context.Context) limits.Limits {
	if q == nil || q.Limits == nil {
		return limits.Defaults
	}
	return q.Limits.Get(ctx)
}

// Charge counts one costly search for the viewer, or returns ErrInviteOnly or
// ErrQuotaExceeded.
func (q *Quotas) Charge(ctx context.Context, v Viewer) error {
	l := q.current(ctx)
	if !Invited(l, v) {
		return ErrInviteOnly
	}
	subject, limit := v.quotaSubject(l)
	_, err := store.New(q.DB).ChargeQuota(ctx, store.ChargeQuotaParams{Subject: subject, QuotaLimit: int32(limit)}) //nolint:gosec // small constant
	if errors.Is(err, pgx.ErrNoRows) {
		return &QuotaError{Limit: limit, Member: v.UserID != ""}
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
