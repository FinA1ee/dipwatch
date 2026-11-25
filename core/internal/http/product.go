package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"github.com/FinA1ee/dipwatch/core/internal/search"
)

// Products serves the product page.
type Products interface {
	Product(ctx context.Context, productID int64, currency string, listing int64) (search.ProductPage, error)
}

type ProductInput struct {
	ProductID int64  `path:"productId" minimum:"1"`
	Currency  string `query:"currency" pattern:"^[A-Z]{3}$" default:"CNY"`
	Listing   int64  `query:"listing" minimum:"0" doc:"Offer id: price history of this listing only; 0: the lowest of all listings"`
}

type ProductOutput struct {
	Body search.ProductPage
}

func registerProducts(api huma.API, p Products) {
	huma.Register(api, huma.Operation{
		OperationID: "get-product",
		Method:      http.MethodGet,
		Path:        "/products/{productId}",
		Summary:     "Product page: offers and daily lowest price",
		Description: "Reads stored data only; it never starts a refresh.",
		Tags:        []string{"Search"},
		Errors:      []int{http.StatusNotFound},
	}, func(ctx context.Context, in *ProductInput) (*ProductOutput, error) {
		page, err := p.Product(ctx, in.ProductID, in.Currency, in.Listing)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("No such product.")
		}
		if err != nil {
			return nil, err
		}
		return &ProductOutput{Body: page}, nil
	})
}
