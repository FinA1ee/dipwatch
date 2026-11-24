// Package sources defines how offers are fetched from stores. Each store (Source)
// has an ordered list of strategies; the Registry tries them cheapest first.
package sources

import (
	"context"
	"encoding/json"
)

// Offer is one listing as a strategy returned it, before LLM matching.
type Offer struct {
	ExternalID    string
	URL           string
	Title         string
	PriceMinor    int64  // minor units of Currency, see package money
	Currency      string // ISO 4217
	ShippingMinor *int64 // nil when unknown
	Condition     string // new, used, refurbished, unknown
	Availability  string // in_stock, out_of_stock, unknown
	SellerType    string // official, third_party, individual, unknown
	Raw           json.RawMessage
}

// Query is what to search for.
type Query struct {
	Product string // canonical product name, e.g. "NVIDIA GeForce RTX 5090"
	Limit   int
}

// Strategy is one way of fetching offers from one store.
type Strategy interface {
	// Name is one of "api", "mcp", "html", "agent".
	Name() string
	Search(ctx context.Context, q Query) ([]Offer, error)
}

// Source is a store with its strategies in the order they should be tried.
type Source struct {
	ID         string
	Strategies []Strategy
}

// Result is what a successful search returned and which strategy produced it.
// The strategy name is for logs and metrics only; users never see it.
type Result struct {
	Strategy string
	Offers   []Offer
}
