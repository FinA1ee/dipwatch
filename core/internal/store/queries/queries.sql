-- name: GetQuery :one
SELECT q.product_id, p.canonical_name
FROM app.queries q
LEFT JOIN app.products p ON p.id = q.product_id
WHERE q.normalized_text = @normalized_text;

-- name: SaveQuery :exec
INSERT INTO app.queries (normalized_text, product_id)
VALUES (@normalized_text, sqlc.narg('product_id'))
ON CONFLICT (normalized_text) DO UPDATE SET product_id = EXCLUDED.product_id;

-- name: AddProductAlias :exec
UPDATE app.products SET aliases = array_append(aliases, @alias::text)
WHERE id = @id AND NOT (@alias::text = ANY(aliases));

-- name: FindProductCandidates :many
-- Existing products sharing a word with the query, so the intent check can reuse
-- a canonical name instead of inventing a second spelling.
SELECT id, canonical_name FROM app.products
WHERE canonical_name ILIKE ANY(@patterns::text[])
ORDER BY id
LIMIT 8;
