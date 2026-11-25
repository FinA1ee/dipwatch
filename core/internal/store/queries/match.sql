-- name: GetMatchDecisions :many
SELECT title_hash, is_product, title_en, condition, variant
FROM app.match_cache
WHERE product_id = @product_id AND title_hash = ANY(@title_hashes::text[]);

-- name: SaveMatchDecision :exec
INSERT INTO app.match_cache (product_id, title_hash, is_product, title_en, condition, variant)
VALUES (@product_id, @title_hash, @is_product, @title_en, @condition, @variant)
ON CONFLICT (product_id, title_hash) DO UPDATE SET
    is_product = EXCLUDED.is_product,
    title_en = EXCLUDED.title_en,
    condition = EXCLUDED.condition,
    variant = EXCLUDED.variant;

-- name: SetOfferMatch :exec
UPDATE app.offers SET is_product = @is_product, title_en = @title_en, condition = @condition, variant = @variant
WHERE id = @id;
