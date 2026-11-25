package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/FinA1ee/dipwatch/core/internal/limits"
	"github.com/FinA1ee/dipwatch/core/internal/spend"
)

// LimitStore reads and changes the owner's budget limits.
type LimitStore interface {
	Get(ctx context.Context) limits.Limits
	Save(ctx context.Context, l limits.Limits) (limits.Limits, error)
}

type LimitsOutput struct{ Body limits.Limits }

// SpendStatus reports today's LLM spend against the budget.
type SpendStatus interface {
	Status(ctx context.Context) (spend.Status, error)
}

type SpendOutput struct{ Body spend.Status }

type SaveLimitsInput struct{ Body limits.Limits }

func registerLimits(api huma.API, store LimitStore, sp SpendStatus) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-spend", Method: http.MethodGet, Path: "/admin/spend",
		Summary: "Today's LLM spend against the daily budget (China day)", Tags: []string{"Admin"},
		Errors: []int{http.StatusForbidden},
	}, func(ctx context.Context, _ *struct{}) (*SpendOutput, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		st, err := sp.Status(ctx)
		if err != nil {
			return nil, err
		}
		return &SpendOutput{Body: st}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-limits", Method: http.MethodGet, Path: "/admin/limits",
		Summary: "Budget limits", Tags: []string{"Admin"}, Errors: []int{http.StatusForbidden},
	}, func(ctx context.Context, _ *struct{}) (*LimitsOutput, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		return &LimitsOutput{Body: store.Get(ctx)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-save-limits", Method: http.MethodPut, Path: "/admin/limits",
		Summary: "Change budget limits", Description: "Applies to the next request in core-api and within 10 seconds in the worker.",
		Tags: []string{"Admin"}, Errors: []int{http.StatusForbidden, http.StatusBadRequest},
	}, func(ctx context.Context, in *SaveLimitsInput) (*LimitsOutput, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		l, err := store.Save(ctx, in.Body)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &LimitsOutput{Body: l}, nil
	})
}
