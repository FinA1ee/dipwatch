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
-- Tracked products with stores whose re-check is due and that are not
-- refreshing (T3). Each tracker is due at its store when that store was last
-- refreshed longer ago than the tracker's interval: the automatic one, or the
-- one it chose, but never under the owner's minimum (min_check_seconds). A
-- search at another store does not delay it. sources lists only the due
-- stores; all_sources is a tracker from before per-store tracking (every
-- enabled store), due by the product's last refresh. Untracked products are
-- never scheduled. (Postgres's GREATEST ignores NULL, so "automatic" needs
-- its own branch: GREATEST(NULL, 1 hour) would be 1 hour.)
SELECT id, sources, all_sources FROM (
    SELECT p.id, p.last_refreshed_at,
           ARRAY(
               SELECT DISTINCT s.src
               FROM app.watch_items w
               CROSS JOIN LATERAL unnest(w.sources) AS s(src)
               LEFT JOIN app.product_sources ps ON ps.product_id = p.id AND ps.source_id = s.src
               WHERE w.product_id = p.id
                 AND (ps.last_refreshed_at IS NULL OR ps.last_refreshed_at + CASE
                      WHEN w.check_every IS NULL THEN p.refresh_interval -- automatic
                      ELSE LEAST(p.refresh_interval, GREATEST(w.check_every, make_interval(secs => @min_check_seconds::int)))
                      END <= now())
               ORDER BY 1
           )::text[] AS sources,
           EXISTS (
               SELECT 1 FROM app.watch_items w
               WHERE w.product_id = p.id AND cardinality(w.sources) = 0
                 AND (p.last_refreshed_at IS NULL OR p.last_refreshed_at + CASE
                      WHEN w.check_every IS NULL THEN p.refresh_interval -- automatic
                      ELSE LEAST(p.refresh_interval, GREATEST(w.check_every, make_interval(secs => @min_check_seconds::int)))
                      END <= now())
           ) AS all_sources
    FROM app.products p
    WHERE cardinality(p.refreshing_sources) = 0
      AND EXISTS (SELECT 1 FROM app.watch_items w WHERE w.product_id = p.id)
) due
WHERE cardinality(sources) > 0 OR all_sources
ORDER BY last_refreshed_at NULLS FIRST
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
