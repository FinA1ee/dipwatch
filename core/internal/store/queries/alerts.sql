-- name: InsertAlert :one
-- One alert per tracker, offer, rule and channel per day (T5); a repeat returns no row.
INSERT INTO app.alerts (watch_item_id, offer_id, rule, channel, alert_date, previous_cny_minor, current_cny_minor)
VALUES (@watch_item_id, @offer_id, @rule, @channel, (now() AT TIME ZONE 'UTC')::date,
        sqlc.narg('previous_cny_minor'), sqlc.narg('current_cny_minor'))
ON CONFLICT DO NOTHING
RETURNING id;

-- name: GetAlertToSend :one
SELECT a.id, a.rule, a.channel, a.status, a.previous_cny_minor, a.current_cny_minor,
       o.title_en, o.title_original, o.url, o.price_minor, o.currency,
       s.name AS source_name, p.id AS product_id, p.canonical_name,
       u.email, u.feishu_webhook
FROM app.alerts a
JOIN app.watch_items w ON w.id = a.watch_item_id
JOIN app.users u ON u.id = w.user_id
JOIN app.offers o ON o.id = a.offer_id
JOIN app.sources s ON s.id = o.source_id
JOIN app.products p ON p.id = o.product_id
WHERE a.id = @id;

-- name: MarkAlert :exec
UPDATE app.alerts SET status = @status, sent_at = CASE WHEN @status = 'sent' THEN now() END WHERE id = @id;

-- name: LatestPricePoint :one
SELECT price_minor, currency, in_stock FROM app.price_points
WHERE offer_id = @offer_id
ORDER BY observed_at DESC, id DESC
LIMIT 1;

-- name: SourceSeenBefore :one
-- Whether the product had an offer from the source before the given time (rule "new source").
SELECT EXISTS (
    SELECT 1 FROM app.offers
    WHERE product_id = @product_id AND source_id = @source_id AND first_seen_at < @before
)::bool;

-- name: ListAlertOffers :many
-- The product's offers that the matcher did not reject, from every store.
SELECT id, source_id, price_minor, currency, availability, first_seen_at, variant, price_unconfirmed
FROM app.offers
WHERE product_id = @product_id AND is_product IS NOT FALSE;

-- name: SetWatchLastPrice :exec
-- The tracked price a refresh last saw, for rule "change".
UPDATE app.watch_items SET last_price_cny_minor = @price WHERE id = @id;
