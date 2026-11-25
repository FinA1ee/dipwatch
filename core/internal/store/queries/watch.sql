-- name: ListWatchItems :many
SELECT w.*, p.canonical_name
FROM app.watch_items w
JOIN app.products p ON p.id = w.product_id
WHERE w.user_id = @user_id
ORDER BY w.created_at DESC;

-- name: CountWatchItems :one
SELECT count(*)::int FROM app.watch_items WHERE user_id = @user_id;

-- name: GetWatchItem :one
SELECT * FROM app.watch_items WHERE user_id = @user_id AND product_id = @product_id;

-- name: SaveWatchItem :one
-- Creates the tracker, or updates its note and rules. The baseline price is
-- set once, when tracking starts (rule "drop by a percentage since tracking started").
INSERT INTO app.watch_items (user_id, product_id, note, rules, baseline_price_cny_minor)
VALUES (@user_id, @product_id, @note, @rules, sqlc.narg('baseline_price_cny_minor'))
ON CONFLICT (user_id, product_id) DO UPDATE SET note = EXCLUDED.note, rules = EXCLUDED.rules
RETURNING *;

-- name: DeleteWatchItem :execrows
DELETE FROM app.watch_items WHERE user_id = @user_id AND product_id = @product_id;

-- name: ListProductWatchers :many
-- Trackers of a product with their member's channels, for alert evaluation.
SELECT w.id, w.user_id, w.rules, w.baseline_price_cny_minor, w.created_at,
       u.email, u.feishu_webhook, u.email_alerts
FROM app.watch_items w
JOIN app.users u ON u.id = w.user_id
WHERE w.product_id = @product_id;
