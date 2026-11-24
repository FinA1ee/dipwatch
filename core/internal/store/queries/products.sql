-- name: UpsertProduct :one
INSERT INTO app.products (canonical_name, category)
VALUES (@canonical_name, @category)
ON CONFLICT (canonical_name) DO UPDATE SET category = EXCLUDED.category
RETURNING *;

-- name: GetProduct :one
SELECT * FROM app.products WHERE id = @id;

-- name: MarkProductRefreshed :exec
UPDATE app.products SET last_refreshed_at = now() WHERE id = @id;
