package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testToken = "test-token-0123456789-0123456789-abcdef"

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func do(t *testing.T, h http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set(InternalTokenHeader, token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthReportsDatabaseStatus(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantCode   int
		wantStatus string
		wantDB     string
	}{
		{"db up", nil, http.StatusOK, "ok", "ok"},
		{"db down", errors.New("connection refused"), http.StatusServiceUnavailable, "degraded", "down"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := New(fakePinger{err: tt.pingErr}, testToken, nil)
			rec := do(t, h, "/healthz", "")

			if rec.Code != tt.wantCode {
				t.Fatalf("code: got %d, want %d", rec.Code, tt.wantCode)
			}
			var body struct {
				Status string `json:"status"`
				DB     string `json:"db"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body %q: %v", rec.Body.String(), err)
			}
			if body.Status != tt.wantStatus || body.DB != tt.wantDB {
				t.Fatalf("body: got %+v, want status=%s db=%s", body, tt.wantStatus, tt.wantDB)
			}
		})
	}
}

func TestInternalTokenRequiredExceptHealth(t *testing.T) {
	h, _ := New(fakePinger{}, testToken, nil)

	tests := []struct {
		name     string
		path     string
		token    string
		wantCode int
	}{
		{"health without token", "/healthz", "", http.StatusOK},
		{"api without token", "/openapi.json", "", http.StatusUnauthorized},
		{"api with wrong token", "/openapi.json", "wrong", http.StatusUnauthorized},
		{"api with token", "/openapi.json", testToken, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, tt.path, tt.token)
			if rec.Code != tt.wantCode {
				t.Fatalf("code: got %d, want %d (body %s)", rec.Code, tt.wantCode, rec.Body.String())
			}
		})
	}
}
