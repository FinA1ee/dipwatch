package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type HealthOutput struct {
	Status int
	Body   struct {
		Status string `json:"status" enum:"ok,degraded" doc:"ok when every dependency answers"`
		DB     string `json:"db" enum:"ok,down" doc:"Postgres status"`
	}
}

func registerHealth(api huma.API, db Pinger) {
	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/healthz",
		Summary:     "Service health",
		Tags:        []string{"System"},
		Responses: map[string]*huma.Response{
			"503": {Description: "A dependency is down"},
		},
	}, func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		out := &HealthOutput{Status: http.StatusOK}
		out.Body.Status, out.Body.DB = "ok", "ok"
		if err := db.Ping(ctx); err != nil {
			slog.WarnContext(ctx, "health: database ping failed", "err", err)
			out.Status = http.StatusServiceUnavailable
			out.Body.Status, out.Body.DB = "degraded", "down"
		}
		return out, nil
	})
}
