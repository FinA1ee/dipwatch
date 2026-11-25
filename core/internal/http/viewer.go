package httpapi

import (
	"context"
	"net/http"

	"github.com/FinA1ee/dipwatch/core/internal/guard"
)

// Identity headers set by the gateway. It strips any a client sends, and core
// only accepts requests carrying the internal token, so they can be trusted.
const (
	UserIDHeader    = "X-Dipwatch-User-Id"
	UserEmailHeader = "X-Dipwatch-User-Email"
	RoleHeader      = "X-Dipwatch-Role"
	ClientIPHeader  = "X-Dipwatch-Client-Ip"
)

type viewerKey struct{}

// withViewer reads the identity headers into the request context. Kept out of
// the operation inputs so they do not appear in the public API contract.
func withViewer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := guard.Viewer{
			UserID: r.Header.Get(UserIDHeader),
			Email:  r.Header.Get(UserEmailHeader),
			Admin:  r.Header.Get(RoleHeader) == "admin",
			IP:     r.Header.Get(ClientIPHeader),
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), viewerKey{}, v)))
	})
}

func viewerFrom(ctx context.Context) guard.Viewer {
	v, _ := ctx.Value(viewerKey{}).(guard.Viewer)
	return v
}
