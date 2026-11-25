// Package limits holds the budget limits the owner changes at runtime on the
// admin page (task 6.7). Values live in app.settings and apply to the next
// request; unset values use the defaults below.
package limits

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// Limits are the knobs that bound LLM and browser spend.
type Limits struct {
	SourcesPerSearch     int      `json:"sources_per_search" minimum:"1" maximum:"20" doc:"Stores a member or visitor may pick for one search"`
	TrackedPerMember     int      `json:"tracked_per_member" minimum:"0" maximum:"100" doc:"Products one member may track (C2)"`
	VisitorDailySearches int      `json:"visitor_daily_searches" minimum:"0" maximum:"1000" doc:"Costly searches per visitor IP per day (C1)"`
	MemberDailySearches  int      `json:"member_daily_searches" minimum:"0" maximum:"1000" doc:"Costly searches per member per day (C1)"`
	DailyBudgetCNY       float64  `json:"daily_budget_cny" minimum:"0" maximum:"1000" doc:"LLM spend per China day; at 80% only tracked products get agent runs, at 100% only stored data is served (C4)"`
	InviteOnly           bool     `json:"invite_only" doc:"Only invited members (and admins) may run costly searches and track products; everyone can still read stored results"`
	Invited              []string `json:"invited" maxItems:"500" doc:"Email addresses of invited members, as their sign-in account reports them"`
}

// MaxInvited is the longest invite list.
const MaxInvited = 500

// Allows reports whether the email may use costly features. Admins are checked
// by the caller.
func (l Limits) Allows(email string) bool {
	return !l.InviteOnly || (email != "" && slices.Contains(l.Invited, strings.ToLower(strings.TrimSpace(email))))
}

// normalize lower-cases, trims, drops blanks and duplicates, and sorts the invite list.
func (l *Limits) normalize() {
	out := make([]string, 0, len(l.Invited))
	for _, e := range l.Invited {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			out = append(out, e)
		}
	}
	slices.Sort(out)
	l.Invited = slices.Compact(out)
}

// Defaults keep spend low until the owner raises them.
// The site starts invite-only: a saved limits row from before the invite list
// existed has no invite_only field, so it gets this default too.
var Defaults = Limits{SourcesPerSearch: 2, TrackedPerMember: 3, VisitorDailySearches: 10, MemberDailySearches: 30, DailyBudgetCNY: 5, InviteOnly: true, Invited: []string{}}

// Validate checks ranges, matching the API schema, for callers other than the API.
func (l Limits) Validate() error {
	switch {
	case l.SourcesPerSearch < 1 || l.SourcesPerSearch > 20:
		return fmt.Errorf("sources per search must be 1 to 20")
	case l.TrackedPerMember < 0 || l.TrackedPerMember > 100:
		return fmt.Errorf("tracked products per member must be 0 to 100")
	case l.VisitorDailySearches < 0 || l.VisitorDailySearches > 1000, l.MemberDailySearches < 0 || l.MemberDailySearches > 1000:
		return fmt.Errorf("daily searches must be 0 to 1000")
	case l.DailyBudgetCNY < 0 || l.DailyBudgetCNY > 1000:
		return fmt.Errorf("the daily budget must be 0 to 1000 yuan")
	case len(l.Invited) > MaxInvited:
		return fmt.Errorf("the invite list can have up to %d addresses", MaxInvited)
	}
	for _, e := range l.Invited {
		if a, err := mail.ParseAddress(e); err != nil || a.Address != e {
			return fmt.Errorf("%q is not an email address", e)
		}
	}
	return nil
}

const settingsKey = "limits"

// cacheFor bounds how stale a limit may be in another process (core-api and
// worker each cache); changes from the admin page apply in this process at once.
const cacheFor = 10 * time.Second

// Store reads and writes the limits.
type Store struct {
	DB *pgxpool.Pool

	mu      sync.Mutex
	cached  Limits
	fetched time.Time
}

// Get returns the current limits; on a database error, the last known or the defaults.
func (s *Store) Get(ctx context.Context) Limits {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.fetched.IsZero() && time.Since(s.fetched) < cacheFor {
		return s.cached
	}
	l := Defaults
	rows, err := store.New(s.DB).ListSettings(ctx)
	if err != nil {
		if s.fetched.IsZero() {
			return Defaults
		}
		return s.cached
	}
	for _, r := range rows {
		if r.Key == settingsKey {
			_ = json.Unmarshal(r.Value, &l) // fields missing from an older value keep their default
		}
	}
	s.cached, s.fetched = l, time.Now()
	return l
}

// Save stores new limits.
func (s *Store) Save(ctx context.Context, l Limits) (Limits, error) {
	l.normalize()
	if err := l.Validate(); err != nil {
		return Limits{}, err
	}
	raw, err := json.Marshal(l)
	if err != nil {
		return Limits{}, err
	}
	if err := store.New(s.DB).SaveSetting(ctx, store.SaveSettingParams{Key: settingsKey, Value: raw}); err != nil {
		return Limits{}, err
	}
	s.mu.Lock()
	s.cached, s.fetched = l, time.Now()
	s.mu.Unlock()
	return l, nil
}
