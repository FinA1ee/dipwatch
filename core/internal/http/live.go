package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"

	"github.com/FinA1ee/dipwatch/core/internal/live"
)

// LiveFeed is the in-memory record of recent agent runs.
type LiveFeed interface {
	Recent() []live.Run
	Subscribe() (<-chan live.Event, func())
}

// Snapshot is sent first: every remembered run with the steps so far.
type Snapshot struct {
	Runs []live.Run `json:"runs" doc:"Newest first"`
}

// liveStreamLimit ends an idle admin stream; the page reconnects.
const liveStreamLimit = 30 * time.Minute

func registerLive(api huma.API, feed LiveFeed) {
	sse.Register(api, huma.Operation{
		OperationID: "admin-live",
		Method:      http.MethodGet,
		Path:        "/admin/live",
		Summary:     "Stream browsing-agent steps as they happen",
		Description: "Sends `snapshot` with recent runs, then an `event` per start, step and end.",
		Tags:        []string{"Admin"},
		Errors:      []int{http.StatusForbidden},
	}, map[string]any{"snapshot": Snapshot{}, "event": live.Event{}}, func(ctx context.Context, _ *struct{}, send sse.Sender) {
		events, stop := feed.Subscribe()
		defer stop()
		if send.Data(Snapshot{Runs: feed.Recent()}) != nil {
			return
		}
		limit := time.After(liveStreamLimit)
		ping := time.NewTicker(20 * time.Second)
		defer ping.Stop()
		for {
			select {
			case e := <-events:
				if send.Data(e) != nil {
					return
				}
			case <-ping.C:
				if send.Comment("ping") != nil { // keeps proxies from closing an idle stream
					return
				}
			case <-limit:
				return
			case <-ctx.Done():
				return
			}
		}
	})
}

// adminPaths answers 403 for every /admin path unless the gateway marked the
// viewer as admin. Operations check too; this also covers streams, which
// cannot send an error status once they have started.
func adminPaths(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/admin") && !viewerFrom(r.Context()).Admin {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"title":"Forbidden","status":403,"detail":"Admins only."}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
