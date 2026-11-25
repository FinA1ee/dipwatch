package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/FinA1ee/dipwatch/core/internal/alerts"
	"github.com/FinA1ee/dipwatch/core/internal/guard"
	"github.com/FinA1ee/dipwatch/core/internal/notify"
	"github.com/FinA1ee/dipwatch/core/internal/tracking"
)

// Tracker manages a member's tracked products and alert settings.
type Tracker interface {
	List(ctx context.Context, v guard.Viewer) ([]tracking.Item, error)
	Save(ctx context.Context, v guard.Viewer, productID int64, note string, rules []alerts.Rule, stores []string, offerID *int64) (tracking.Item, error)
	Delete(ctx context.Context, v guard.Viewer, productID int64) error
	Settings(ctx context.Context, v guard.Viewer) (tracking.Settings, error)
	SaveSettings(ctx context.Context, v guard.Viewer, feishu string, email bool) (tracking.Settings, error)
	TestFeishu(ctx context.Context, v guard.Viewer) error
}

// Simulator queues a simulated price drop (admin test of alert delivery).
type Simulator interface {
	SimulateDrop(ctx context.Context, productID int64, percent int) error
}

type TrackedOutput struct {
	Body struct {
		Items []tracking.Item `json:"items" doc:"Newest first"`
	}
}

type ProductPath struct {
	ProductID int64 `path:"productId" minimum:"1"`
}

type SaveTrackedInput struct {
	ProductPath
	Body struct {
		Note    string        `json:"note" maxLength:"2000" doc:"Up to 500 characters"`
		Rules   []alerts.Rule `json:"rules" minItems:"1" maxItems:"8"`
		Stores  []string      `json:"stores,omitempty" maxItems:"20" doc:"Stores to track at (needed when tracking starts without a listing; ignored afterwards)"`
		OfferID *int64        `json:"offer_id,omitempty" doc:"Follow this listing only; omit to follow the product's cheapest offer"`
	}
}

type TrackedItemOutput struct{ Body tracking.Item }

type SettingsOutput struct{ Body tracking.Settings }

type SaveSettingsInput struct {
	Body struct {
		FeishuWebhook string `json:"feishu_webhook" maxLength:"500" doc:"Feishu or Lark bot webhook URL; empty turns Feishu alerts off"`
		EmailAlerts   bool   `json:"email_alerts"`
	}
}

type SimulateInput struct {
	ProductPath
	Body struct {
		Percent int `json:"percent" minimum:"1" maximum:"90" default:"20"`
	}
}

func registerTracking(api huma.API, t Tracker, sim Simulator) {
	member := []int{http.StatusUnauthorized, http.StatusBadRequest}
	tags := []string{"Tracking"}

	huma.Register(api, huma.Operation{
		OperationID: "list-tracked", Method: http.MethodGet, Path: "/me/tracked",
		Summary: "Products I track", Tags: tags, Errors: member,
	}, func(ctx context.Context, _ *struct{}) (*TrackedOutput, error) {
		items, err := t.List(ctx, viewerFrom(ctx))
		if err != nil {
			return nil, trackingError(err)
		}
		out := &TrackedOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "save-tracked", Method: http.MethodPut, Path: "/me/tracked/{productId}",
		Summary: "Track a product, or change its note and rules", Tags: tags,
		Errors: append(member, http.StatusForbidden, http.StatusNotFound, http.StatusConflict),
	}, func(ctx context.Context, in *SaveTrackedInput) (*TrackedItemOutput, error) {
		item, err := t.Save(ctx, viewerFrom(ctx), in.ProductID, in.Body.Note, in.Body.Rules, in.Body.Stores, in.Body.OfferID)
		if errors.Is(err, guard.ErrInviteOnly) {
			return nil, inviteError(viewerFrom(ctx))
		}
		if err != nil {
			return nil, trackingError(err)
		}
		return &TrackedItemOutput{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-tracked", Method: http.MethodDelete, Path: "/me/tracked/{productId}",
		Summary: "Stop tracking a product", Tags: tags, DefaultStatus: http.StatusNoContent,
		Errors: append(member, http.StatusNotFound),
	}, func(ctx context.Context, in *ProductPath) (*struct{}, error) {
		if err := t.Delete(ctx, viewerFrom(ctx), in.ProductID); err != nil {
			return nil, trackingError(err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-settings", Method: http.MethodGet, Path: "/me/settings",
		Summary: "My alert channels", Tags: tags, Errors: member,
	}, func(ctx context.Context, _ *struct{}) (*SettingsOutput, error) {
		s, err := t.Settings(ctx, viewerFrom(ctx))
		if err != nil {
			return nil, trackingError(err)
		}
		return &SettingsOutput{Body: s}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "save-settings", Method: http.MethodPut, Path: "/me/settings",
		Summary: "Change my alert channels", Tags: tags, Errors: member,
	}, func(ctx context.Context, in *SaveSettingsInput) (*SettingsOutput, error) {
		s, err := t.SaveSettings(ctx, viewerFrom(ctx), in.Body.FeishuWebhook, in.Body.EmailAlerts)
		if err != nil {
			return nil, trackingError(err)
		}
		return &SettingsOutput{Body: s}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "test-feishu", Method: http.MethodPost, Path: "/me/settings/test",
		Summary: "Send a test message to my Feishu webhook", Tags: tags, DefaultStatus: http.StatusNoContent,
		Errors: append(member, http.StatusBadGateway),
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		if err := t.TestFeishu(ctx, viewerFrom(ctx)); err != nil {
			return nil, trackingError(err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-simulate-drop", Method: http.MethodPost, Path: "/admin/products/{productId}/simulate-drop",
		Summary:     "Lower the cheapest offer and run the alert check",
		Description: "Test of alert delivery (M6.3): changes stored prices, so use it on test products only.",
		Tags:        []string{"Admin"}, DefaultStatus: http.StatusAccepted, Errors: []int{http.StatusForbidden},
	}, func(ctx context.Context, in *SimulateInput) (*struct{}, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		return nil, sim.SimulateDrop(ctx, in.ProductID, in.Body.Percent)
	})
}

func trackingError(err error) error {
	switch {
	case errors.Is(err, tracking.ErrNotMember):
		return huma.Error401Unauthorized(err.Error())
	case errors.Is(err, tracking.ErrNoProduct), errors.Is(err, tracking.ErrNotTracked):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, tracking.ErrNoteTooLong), errors.Is(err, tracking.ErrNoWebhook), errors.Is(err, tracking.ErrStores), errors.Is(err, tracking.ErrListing), errors.Is(err, notify.ErrWebhookHost):
		return huma.Error400BadRequest(err.Error())
	case isRuleError(err):
		return huma.Error400BadRequest(err.Error())
	}
	var limitErr *tracking.LimitError
	if errors.As(err, &limitErr) {
		return huma.Error409Conflict(limitErr.Error())
	}
	var sendErr *notify.SendError
	if errors.As(err, &sendErr) {
		return huma.Error502BadGateway("Feishu did not accept the message: " + sendErr.Error())
	}
	return err
}

// isRuleError reports errors from alerts.Validate, which are all member mistakes.
func isRuleError(err error) bool {
	var re alerts.RuleError
	return errors.As(err, &re)
}

// inviteError tells an uninvited viewer which email to send the owner.
func inviteError(v guard.Viewer) error {
	if v.Email == "" {
		return huma.Error403Forbidden("Dipwatch is invite-only for now. Sign in, then ask the owner to invite your email.")
	}
	return huma.Error403Forbidden(fmt.Sprintf("Dipwatch is invite-only for now. Ask the owner to invite %s.", v.Email))
}
