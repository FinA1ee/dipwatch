-- name: UpsertProduct :one
INSERT INTO app.products (canonical_name, category)
VALUES (@canonical_name, @category)
ON CONFLICT (canonical_name) DO UPDATE SET category = EXCLUDED.category
RETURNING *;

-- name: GetProduct :one
SELECT * FROM app.products WHERE id = @id;

-- name: MarkProductRefreshed :exec
UPDATE app.products SET last_refreshed_at = now() WHERE id = @id;

-- name: StartRefresh :exec
-- Adds the refresh's stores to those already running: searches for different
-- stores of one product may overlap.
UPDATE app.products SET
    refreshing_sources = ARRAY(SELECT DISTINCT unnest(refreshing_sources || @source_ids::text[]) ORDER BY 1),
    refresh_changed = false
WHERE id = @id;

-- name: MarkSourceRefreshed :exec
INSERT INTO app.product_sources (product_id, source_id, last_refreshed_at)
VALUES (@product_id, @source_id, now())
ON CONFLICT (product_id, source_id) DO UPDATE SET last_refreshed_at = now();

-- name: SourceFreshness :many
SELECT source_id, last_refreshed_at FROM app.product_sources
WHERE product_id = @product_id AND source_id = ANY(@source_ids::text[]);

-- name: FinishSourceRefresh :one
-- Removes one finished source and returns how many are still running.
UPDATE app.products SET refreshing_sources = array_remove(refreshing_sources, @source_id::text)
WHERE id = @id
RETURNING cardinality(refreshing_sources)::int AS remaining;

-- name: NotifyOffers :exec
SELECT pg_notify('dipwatch_offers', @payload::text);

-- name: NotifyAgent :exec
SELECT pg_notify('dipwatch_agent', @payload::text);

-- name: DueTrackedProducts :many
-- Tracked products whose refresh interval has passed and that are not refreshing
-- (T3), with the stores their trackers chose. An empty list means a tracker from
-- before per-store tracking: every enabled store. Untracked products are never scheduled.
-- The interval is the automatic one, or the shortest a tracker chose, but never
-- under the owner's minimum (min_check_seconds).
SELECT p.id,
       ARRAY(SELECT DISTINCT s FROM app.watch_items w, unnest(w.sources) s WHERE w.product_id = p.id ORDER BY 1)::text[] AS sources,
       EXISTS (SELECT 1 FROM app.watch_items w WHERE w.product_id = p.id AND cardinality(w.sources) = 0) AS all_sources
FROM app.products p
WHERE EXISTS (SELECT 1 FROM app.watch_items w WHERE w.product_id = p.id)
  AND cardinality(p.refreshing_sources) = 0
  AND (p.last_refreshed_at IS NULL OR p.last_refreshed_at + LEAST(
        p.refresh_interval,
        COALESCE((SELECT GREATEST(min(w.check_every), make_interval(secs => @min_check_seconds::int))
                  FROM app.watch_items w WHERE w.product_id = p.id), p.refresh_interval)
      ) <= now())
ORDER BY p.last_refreshed_at NULLS FIRST
LIMIT 50;

-- name: MarkRefreshChanged :exec
UPDATE app.products SET refresh_changed = true WHERE id = @id;

-- name: AdaptRefreshInterval :exec
-- After a refresh: halve the interval (at least 2 hours) when a known offer
-- changed; double it (at most 24 hours) after 4 refreshes in a row without a change.
UPDATE app.products SET
    refresh_interval = CASE
        WHEN refresh_changed THEN greatest(refresh_interval / 2, interval '2 hours')
        WHEN unchanged_checks + 1 >= 4 THEN least(refresh_interval * 2, interval '24 hours')
        ELSE refresh_interval END,
    unchanged_checks = CASE
        WHEN refresh_changed OR unchanged_checks + 1 >= 4 THEN 0
        ELSE unchanged_checks + 1 END,
    refresh_changed = false
WHERE id = @id;
