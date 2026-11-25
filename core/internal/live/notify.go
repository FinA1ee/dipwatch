package live

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// Notifier sends events with Postgres NOTIFY. A lost event only affects the
// live view; the full trace is still recorded at the end of the run.
type Notifier struct {
	Queries *store.Queries
}

func (n Notifier) Observe(ctx context.Context, e Event) {
	payload, err := json.Marshal(e)
	if err == nil {
		err = n.Queries.NotifyAgent(ctx, string(payload))
	}
	if err != nil {
		slog.WarnContext(ctx, "live: notify failed", "run", e.Run, "kind", e.Kind, "err", err)
	}
}
