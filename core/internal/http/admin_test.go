package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FinA1ee/dipwatch/core/internal/live"
	"github.com/FinA1ee/dipwatch/core/internal/runs"
)

type fakeRuns struct{ listed *runs.Filter }

func (f *fakeRuns) Metrics(context.Context, int) (runs.Metrics, error) {
	return runs.Metrics{Sources: []runs.SourceDay{}, Spend: []runs.SpendDay{}}, nil
}

func (f *fakeRuns) List(_ context.Context, filter runs.Filter) ([]runs.Summary, error) {
	f.listed = &filter
	return []runs.Summary{}, nil
}

func (f *fakeRuns) Get(context.Context, int64) (runs.Detail, error) { return runs.Detail{}, nil }

func TestAdminEndpointsNeedTheAdminRole(t *testing.T) {
	r := &fakeRuns{}
	h, _ := New(fakePinger{}, testToken, Services{Runs: r, Live: &live.Hub{}})
	get := func(path, role string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(InternalTokenHeader, testToken)
		req.Header.Set(UserIDHeader, "u1")
		if role != "" {
			req.Header.Set(RoleHeader, role)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, path := range []string{"/admin/metrics", "/admin/runs", "/admin/runs/1", "/admin/live"} {
		if path == "/admin/live" {
			// A stream answers 200 and stays open, so only the refusals are checked here.
			if code := get(path, ""); code != http.StatusForbidden {
				t.Errorf("%s without role: %d, want 403", path, code)
			}
			continue
		}
		if code := get(path, ""); code != http.StatusForbidden {
			t.Errorf("%s without role: %d, want 403", path, code)
		}
		if code := get(path, "member"); code != http.StatusForbidden {
			t.Errorf("%s with a non-admin role: %d, want 403", path, code)
		}
		if code := get(path, "admin"); code != http.StatusOK {
			t.Errorf("%s as admin: %d, want 200", path, code)
		}
	}

	if code := get("/admin/runs?source=smzdm&outcome=ok&prompt_version=browse-v8&day=2026-11-25&kind=browse", "admin"); code != http.StatusOK {
		t.Fatalf("filtered list: %d", code)
	}
	if f := r.listed; f == nil || f.SourceID != "smzdm" || f.Outcome != "ok" || f.PromptVersion != "browse-v8" || f.Kind != "browse" || f.Day.Day() != 25 {
		t.Fatalf("filter not passed through: %+v", f)
	}
}
