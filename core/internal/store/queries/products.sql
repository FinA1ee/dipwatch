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
UPDATE app.products SET refreshing_sources = @source_ids::text[] WHERE id = @id;

-- name: FinishSourceRefresh :one
-- Removes one finished source and returns how many are still running.
UPDATE app.products SET refreshing_sources = array_remove(refreshing_sources, @source_id::text)
WHERE id = @id
RETURNING cardinality(refreshing_sources)::int AS remaining;

-- name: NotifyOffers :exec
SELECT pg_notify('dipwatch_offers', @payload::text);

-- name: NotifyAgent :exec
SELECT pg_notify('dipwatch_agent', @payload::text);
