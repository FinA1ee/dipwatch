-- name: ListWatchItems :many
-- A pinned listing comes with its current price and when a refresh last saw it.
SELECT w.*, p.canonical_name,
       o.title_original AS offer_title, o.source_id AS offer_source_id, o.url AS offer_url,
       o.price_minor AS offer_price_minor, o.currency AS offer_currency, o.last_seen_at AS offer_last_seen_at,
       o.variant AS offer_variant,
       ps.last_refreshed_at AS offer_source_refreshed_at
FROM app.watch_items w
JOIN app.products p ON p.id = w.product_id
LEFT JOIN app.offers o ON o.id = w.offer_id
LEFT JOIN app.product_sources ps ON ps.product_id = w.product_id AND ps.source_id = o.source_id
WHERE w.user_id = @user_id
ORDER BY w.created_at DESC;

-- name: CountWatchItems :one
SELECT count(*)::int FROM app.watch_items WHERE user_id = @user_id;

-- name: GetWatchItem :one
SELECT * FROM app.watch_items WHERE user_id = @user_id AND product_id = @product_id;

-- name: SaveWatchItem :one
-- Creates or updates the tracker. The service decides the baseline price and
-- stores: they are set when tracking starts and again when the pinned listing
-- changes, and otherwise passed through unchanged.
INSERT INTO app.watch_items (user_id, product_id, note, rules, baseline_price_cny_minor, sources, offer_id, variant)
VALUES (@user_id, @product_id, @note, @rules, sqlc.narg('baseline_price_cny_minor'), @sources::text[], sqlc.narg('offer_id'), @variant)
ON CONFLICT (user_id, product_id) DO UPDATE
SET note = EXCLUDED.note, rules = EXCLUDED.rules, baseline_price_cny_minor = EXCLUDED.baseline_price_cny_minor,
    sources = EXCLUDED.sources, offer_id = EXCLUDED.offer_id, variant = EXCLUDED.variant
RETURNING *;

-- name: DeleteWatchItem :execrows
DELETE FROM app.watch_items WHERE user_id = @user_id AND product_id = @product_id;

-- name: ListProductWatchers :many
-- Trackers of a product with their member's channels, for alert evaluation.
SELECT w.id, w.user_id, w.rules, w.baseline_price_cny_minor, w.created_at, w.sources, w.offer_id, w.variant,
       u.email, u.feishu_webhook, u.email_alerts
FROM app.watch_items w
JOIN app.users u ON u.id = w.user_id
WHERE w.product_id = @product_id;

-- name: ProductTracked :one
SELECT EXISTS (SELECT 1 FROM app.watch_items WHERE product_id = @product_id)::bool;
