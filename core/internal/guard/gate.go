package guard

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// ErrUnavailable means a query was not in the cache and no model is configured.
var ErrUnavailable = errors.New("intent check unavailable")

// Gate resolves search text to a catalog product.
type Gate struct {
	DB      *pgxpool.Pool
	Checker *Checker // nil: only cached queries resolve
}

// Product is a resolved query.
type Product struct {
	ID            int64
	CanonicalName string
	Cached        bool // true when no LLM call was made
}

// Resolve checks the input, then answers from the queries cache or, on a miss,
// with one intent call whose verdict is cached for every later identical query.
// The intent call is charged to the viewer's quota first; charge may be nil.
func (g *Gate) Resolve(ctx context.Context, text string, charge *Charger) (Product, error) {
	query, err := Normalize(text)
	if err != nil {
		return Product{}, err
	}
	q := store.New(g.DB)
	row, err := q.GetQuery(ctx, query)
	switch {
	case err == nil:
		if row.ProductID == nil || row.CanonicalName == nil {
			return Product{}, ErrNotProduct
		}
		return Product{ID: *row.ProductID, CanonicalName: *row.CanonicalName, Cached: true}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Product{}, err
	}
	if g.Checker == nil {
		return Product{}, ErrUnavailable
	}
	if err := charge.Charge(ctx); err != nil {
		return Product{}, err
	}

	candidates, err := q.FindProductCandidates(ctx, CandidatePatterns(query))
	if err != nil {
		return Product{}, err
	}
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.CanonicalName
	}
	verdict, usage, err := g.Checker.Check(ctx, query, names)
	g.record(ctx, usage, err)
	if err != nil {
		return Product{}, err
	}
	if !verdict.IsProduct {
		if err := q.SaveQuery(ctx, store.SaveQueryParams{NormalizedText: query}); err != nil {
			return Product{}, err
		}
		return Product{}, ErrNotProduct
	}

	product, err := g.catalogProduct(ctx, q, candidates, verdict)
	if err != nil {
		return Product{}, err
	}
	if err := q.SaveQuery(ctx, store.SaveQueryParams{NormalizedText: query, ProductID: &product.ID}); err != nil {
		return Product{}, err
	}
	if err := q.AddProductAlias(ctx, store.AddProductAliasParams{ID: product.ID, Alias: query}); err != nil {
		return Product{}, err
	}
	return product, nil
}

// catalogProduct reuses an existing product when the model returned its name
// (ignoring case), so "rtx 5090" and "RTX 5090" never become two products.
func (g *Gate) catalogProduct(ctx context.Context, q *store.Queries, candidates []store.FindProductCandidatesRow, v Verdict) (Product, error) {
	for _, c := range candidates {
		if strings.EqualFold(c.CanonicalName, v.CanonicalName) {
			return Product{ID: c.ID, CanonicalName: c.CanonicalName}, nil
		}
	}
	p, err := q.UpsertProduct(ctx, store.UpsertProductParams{CanonicalName: v.CanonicalName, Category: v.Category})
	if err != nil {
		return Product{}, err
	}
	return Product{ID: p.ID, CanonicalName: p.CanonicalName}, nil
}

func (g *Gate) record(ctx context.Context, u Usage, callErr error) {
	outcome := "ok"
	if callErr != nil {
		outcome = "error"
	}
	trace, _ := json.Marshal(map[string]any{"prompt_version": PromptVersion})
	if _, err := store.New(g.DB).InsertAgentRun(ctx, store.InsertAgentRunParams{
		Kind: "intent", Model: g.Checker.Model.Name(), Steps: 1,
		TokensIn: int32(min(u.TokensIn, 1<<31-1)), TokensOut: int32(min(u.TokensOut, 1<<31-1)), //nolint:gosec // bounded
		CostCnyMicros: u.CostMicros, DurationMs: int32(min(u.Duration.Milliseconds(), 1<<31-1)), //nolint:gosec // bounded
		Outcome: outcome, Trace: trace,
	}); err != nil {
		slog.WarnContext(ctx, "intent: recording run failed", "err", err)
	}
}
