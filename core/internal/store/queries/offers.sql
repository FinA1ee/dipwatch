-- name: UpsertOffer :one
-- One row per listing (source_id, external_id). Re-seeing a listing updates it in place.
INSERT INTO app.offers (
    product_id, source_id, external_id, url, title_original,
    price_minor, currency, shipping_minor, condition, availability, seller_type, raw, price_unconfirmed
) VALUES (
    @product_id, @source_id, @external_id, @url, @title_original,
    @price_minor, @currency, @shipping_minor, @condition, @availability, @seller_type, @raw, @price_unconfirmed
)
ON CONFLICT (source_id, external_id) DO UPDATE SET
    url = EXCLUDED.url,
    title_original = EXCLUDED.title_original,
    price_minor = EXCLUDED.price_minor,
    currency = EXCLUDED.currency,
    shipping_minor = EXCLUDED.shipping_minor,
    condition = EXCLUDED.condition,
    availability = EXCLUDED.availability,
    seller_type = EXCLUDED.seller_type,
    raw = EXCLUDED.raw,
    price_unconfirmed = EXCLUDED.price_unconfirmed,
    last_seen_at = now()
RETURNING id;

-- name: InsertPricePointIfChanged :execrows
-- Records a price point only when price, currency or stock differ from the latest one,
-- so unchanged re-checks add no rows.
INSERT INTO app.price_points (offer_id, price_minor, currency, in_stock)
SELECT @offer_id::bigint, @price_minor::bigint, @currency::char(3), sqlc.narg('in_stock')::boolean
WHERE NOT EXISTS (
    SELECT 1 FROM (
        SELECT pp.price_minor, pp.currency, pp.in_stock
        FROM app.price_points pp
        WHERE pp.offer_id = @offer_id::bigint
        ORDER BY pp.observed_at DESC, pp.id DESC
        LIMIT 1
    ) latest
    WHERE latest.price_minor = @price_minor::bigint
      AND latest.currency = @currency::char(3)
      AND latest.in_stock IS NOT DISTINCT FROM sqlc.narg('in_stock')::boolean
);

-- name: ListOffersByProduct :many
SELECT * FROM app.offers WHERE product_id = @product_id ORDER BY price_minor, id;

-- name: CountPricePointsByProduct :one
SELECT count(*) FROM app.price_points pp
JOIN app.offers o ON o.id = pp.offer_id
WHERE o.product_id = @product_id;

-- name: ListSearchOffers :many
-- Offers shown in search results: everything the matcher did not reject (S6),
-- from the given stores (empty: every store).
SELECT o.id, o.source_id, s.name AS source_name, s.region AS source_region,
       o.url, o.title_original, o.title_en, o.price_minor, o.currency, o.shipping_minor,
       o.condition, o.availability, o.seller_type, o.variant, o.price_unconfirmed
FROM app.offers o
JOIN app.sources s ON s.id = o.source_id
WHERE o.product_id = @product_id
  AND o.is_product IS NOT FALSE
  AND (cardinality(@source_ids::text[]) = 0 OR o.source_id = ANY(@source_ids::text[]))
ORDER BY o.id;

-- name: ListProductPricePoints :many
-- Price history for the product page: every price point of offers of one
-- variant that the matcher did not reject and whose price the store confirms,
-- since a start time. Converted and reduced per day in Go.
SELECT pp.observed_at, pp.price_minor, pp.currency
FROM app.price_points pp
JOIN app.offers o ON o.id = pp.offer_id
WHERE o.product_id = @product_id
  AND o.is_product IS NOT FALSE
  AND NOT o.price_unconfirmed
  AND (@variant::text = '' OR o.variant = @variant)
  AND (cardinality(@source_ids::text[]) = 0 OR o.source_id = ANY(@source_ids::text[]))
  AND pp.observed_at >= @since
ORDER BY pp.observed_at;

-- name: SetOfferPrice :exec
-- Used only by the simulated price drop (admin test of alert delivery).
UPDATE app.offers SET price_minor = @price_minor WHERE id = @id;

-- name: GetProductOffer :one
-- An offer of the product, for pinning a listing to a tracker.
SELECT id, source_id, price_minor, currency, is_product
FROM app.offers
WHERE id = @id AND product_id = @product_id;

-- name: ListOfferPricePoints :many
-- Price history of one listing of the product (a pinned offer), since a start time.
SELECT pp.observed_at, pp.price_minor, pp.currency
FROM app.price_points pp
JOIN app.offers o ON o.id = pp.offer_id
WHERE pp.offer_id = @offer_id AND o.product_id = @product_id AND pp.observed_at >= @since
ORDER BY pp.observed_at;
