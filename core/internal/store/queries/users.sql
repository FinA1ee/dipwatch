-- name: UpsertUser :exec
-- Members are created by Better Auth; core keeps its own row for settings and
-- foreign keys, created on the member's first request that needs it.
INSERT INTO app.users (id, email) VALUES (@id, @email)
ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email;

-- name: GetUser :one
SELECT * FROM app.users WHERE id = @id;

-- name: SetUserChannels :exec
UPDATE app.users SET feishu_webhook = @feishu_webhook, email_alerts = @email_alerts WHERE id = @id;

-- name: DeleteUser :exec
-- Removes a member's core data (watch items and alerts cascade). Used by tests.
DELETE FROM app.users WHERE id = @id;
