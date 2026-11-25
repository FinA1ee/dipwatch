-- name: UpsertOffer :one
-- One row per listing (source_id, external_id). Re-seeing a listing updates it in place.
INSERT INTO app.offers (
    product_id, source_id, external_id, url, title_original,
    price_minor, currency, shipping_minor, condition, availability, seller_type, raw
) VALUES (
    @product_id, @source_id, @external_id, @url, @title_original,
    @price_minor, @currency, @shipping_minor, @condition, @availability, @seller_type, @raw
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
-- Offers shown in search results: everything the matcher did not reject (S6).
-- Optionally limited to one source, for the per-source stream events.
SELECT o.id, o.source_id, s.name AS source_name, s.region AS source_region,
       o.url, o.title_original, o.title_en, o.price_minor, o.currency, o.shipping_minor,
       o.condition, o.availability, o.seller_type
FROM app.offers o
JOIN app.sources s ON s.id = o.source_id
WHERE o.product_id = @product_id
  AND o.is_product IS NOT FALSE
  AND (sqlc.narg('source_id')::text IS NULL OR o.source_id = sqlc.narg('source_id')::text)
ORDER BY o.id;
