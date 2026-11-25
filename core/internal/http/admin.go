package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"github.com/FinA1ee/dipwatch/core/internal/runs"
)

// Runs reads agent runs for the admin pages.
type Runs interface {
	Metrics(ctx context.Context, days int) (runs.Metrics, error)
	List(ctx context.Context, f runs.Filter) ([]runs.Summary, error)
	Get(ctx context.Context, id int64) (runs.Detail, error)
}

type MetricsInput struct {
	Days int `query:"days" minimum:"1" maximum:"30" default:"7"`
}

type MetricsOutput struct{ Body runs.Metrics }

type RunsInput struct {
	Kind          string `query:"kind" enum:"browse,match,intent,"`
	SourceID      string `query:"source"`
	Outcome       string `query:"outcome"`
	PromptVersion string `query:"prompt_version"`
	Day           string `query:"day" format:"date" doc:"UTC day, YYYY-MM-DD"`
	BeforeID      int64  `query:"before" minimum:"0" doc:"Paging: runs older than this id"`
	Limit         int    `query:"limit" minimum:"1" maximum:"200" default:"50"`
}

type RunsOutput struct {
	Body struct {
		Runs []runs.Summary `json:"runs" doc:"Newest first"`
	}
}

type RunInput struct {
	ID int64 `path:"runId" minimum:"1"`
}

type RunOutput struct{ Body runs.Detail }

func registerAdmin(api huma.API, r Runs) {
	admin := []int{http.StatusForbidden}

	huma.Register(api, huma.Operation{
		OperationID: "admin-metrics", Method: http.MethodGet, Path: "/admin/metrics",
		Summary: "Per-source agent metrics and LLM spend per day", Tags: []string{"Admin"}, Errors: admin,
	}, func(ctx context.Context, in *MetricsInput) (*MetricsOutput, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		m, err := r.Metrics(ctx, in.Days)
		if err != nil {
			return nil, err
		}
		return &MetricsOutput{Body: m}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-runs", Method: http.MethodGet, Path: "/admin/runs",
		Summary: "List agent runs", Tags: []string{"Admin"}, Errors: append(admin, http.StatusBadRequest),
	}, func(ctx context.Context, in *RunsInput) (*RunsOutput, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		f := runs.Filter{Kind: in.Kind, SourceID: in.SourceID, Outcome: in.Outcome, PromptVersion: in.PromptVersion, BeforeID: in.BeforeID, Limit: in.Limit}
		if in.Day != "" {
			day, err := time.Parse(time.DateOnly, in.Day)
			if err != nil {
				return nil, huma.Error400BadRequest("day must be YYYY-MM-DD")
			}
			f.Day = day
		}
		list, err := r.List(ctx, f)
		if err != nil {
			return nil, err
		}
		out := &RunsOutput{}
		out.Body.Runs = list
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-run", Method: http.MethodGet, Path: "/admin/runs/{runId}",
		Summary: "One agent run with its trace", Tags: []string{"Admin"}, Errors: append(admin, http.StatusNotFound),
	}, func(ctx context.Context, in *RunInput) (*RunOutput, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		d, err := r.Get(ctx, in.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("No such run.")
		}
		if err != nil {
			return nil, err
		}
		return &RunOutput{Body: d}, nil
	})
}

func requireAdmin(ctx context.Context) error {
	if !viewerFrom(ctx).Admin {
		return huma.Error403Forbidden("Admins only.")
	}
	return nil
}
