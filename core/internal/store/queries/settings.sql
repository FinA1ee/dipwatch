-- name: ListSettings :many
SELECT key, value FROM app.settings;

-- name: SaveSetting :exec
INSERT INTO app.settings (key, value) VALUES (@key, @value)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();
