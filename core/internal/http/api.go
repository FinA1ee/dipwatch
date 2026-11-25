// Package httpapi defines the core HTTP API. Its operations are the source of the OpenAPI contract.
package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// InternalTokenHeader carries the shared secret the gateway sends on every request.
const InternalTokenHeader = "X-Dipwatch-Internal" //nolint:gosec // header name, not a credential

const (
	Title   = "Dipwatch Core API"
	Version = "0.1.0"
)

// Pinger reports whether the database answers.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Services answer the operations. Fields may be nil when only the contract is needed.
type Services struct {
	Search   Searcher
	Products Products
	Runs     Runs
	Live     LiveFeed
	Tracking Tracker
	Simulate Simulator
}

// New builds the router and registers every operation.
func New(db Pinger, internalToken string, s Services) (http.Handler, huma.API) {
	// Every list core returns is a real (possibly empty) array, never null.
	huma.DefaultArrayNullable = false
	router := chi.NewMux()
	router.Use(requireInternalToken(internalToken), withViewer, adminPaths)

	api := humachi.New(router, huma.DefaultConfig(Title, Version))
	registerHealth(api, db)
	registerSearch(api, s.Search)
	registerProducts(api, s.Products)
	registerAdmin(api, s.Runs)
	registerLive(api, s.Live)
	registerTracking(api, s.Tracking, s.Simulate)
	return router, api
}

// requireInternalToken rejects requests that did not come through the gateway.
// /healthz stays open so container health checks work without the secret.
func requireInternalToken(token string) func(http.Handler) http.Handler {
	want := []byte(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				next.ServeHTTP(w, r)
				return
			}
			got := []byte(r.Header.Get(InternalTokenHeader))
			if subtle.ConstantTimeCompare(got, want) != 1 {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"title":"Unauthorized","status":401}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
