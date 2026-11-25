package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/jackc/pgx/v5"

	"github.com/FinA1ee/dipwatch/core/internal/guard"
	"github.com/FinA1ee/dipwatch/core/internal/search"
)

// Searcher answers searches and streams refresh results.
type Searcher interface {
	Search(ctx context.Context, text, currency string) (search.Result, error)
	Stream(ctx context.Context, productID int64, currency string, emit search.Emit) error
}

type SearchInput struct {
	Body struct {
		Query    string `json:"query" maxLength:"200" doc:"Product to search for, e.g. RTX 5090. At most 80 characters after trimming."`
		Currency string `json:"currency,omitempty" pattern:"^[A-Z]{3}$" default:"CNY" doc:"Currency for converted prices"`
	}
}

type SearchOutput struct {
	Body search.Result
}

type EventsInput struct {
	ProductID int64  `path:"productId" minimum:"1"`
	Currency  string `query:"currency" pattern:"^[A-Z]{3}$" default:"CNY"`
}

func registerSearch(api huma.API, s Searcher) {
	huma.Register(api, huma.Operation{
		OperationID: "search",
		Method:      http.MethodPost,
		Path:        "/search",
		Summary:     "Search offers for a product",
		Description: "Resolves the query to one product and returns its offers. When they are older than 6 hours a refresh starts; follow it with the events stream.",
		Tags:        []string{"Search"},
		Errors:      []int{http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusServiceUnavailable},
	}, func(ctx context.Context, in *SearchInput) (*SearchOutput, error) {
		res, err := s.Search(ctx, in.Body.Query, in.Body.Currency)
		switch {
		case err == nil:
			return &SearchOutput{Body: res}, nil
		case errors.Is(err, guard.ErrEmpty), errors.Is(err, guard.ErrTooLong), errors.Is(err, guard.ErrURL):
			return nil, huma.Error400BadRequest(err.Error())
		case errors.Is(err, guard.ErrNotProduct):
			return nil, huma.Error422UnprocessableEntity("The query does not name a product. Try a model, e.g. RTX 5090.")
		default:
			slog.ErrorContext(ctx, "search failed", "err", err)
			return nil, huma.Error503ServiceUnavailable("Search is unavailable, try again later.")
		}
	})

	sse.Register(api, huma.Operation{
		OperationID: "search-events",
		Method:      http.MethodGet,
		Path:        "/search/{productId}/events",
		Summary:     "Stream offers while sources finish",
		Description: "Sends the full ranked offer list now and after each source finishes, then `done`.",
		Tags:        []string{"Search"},
	}, search.Events, func(ctx context.Context, in *EventsInput, send sse.Sender) {
		err := s.Stream(ctx, in.ProductID, in.Currency, func(event any) error { return send.Data(event) })
		if errors.Is(err, pgx.ErrNoRows) {
			_ = send.Data(search.DoneEvent{})
		} else if err != nil {
			slog.ErrorContext(ctx, "search stream failed", "product", in.ProductID, "err", err)
			_ = send.Data(search.DoneEvent{TimedOut: true})
		}
	})
}
