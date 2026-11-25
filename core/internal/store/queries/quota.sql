-- name: ChargeQuota :one
-- Counts one costly search for the subject today, unless the limit is reached.
-- No row returned means the subject is over its limit.
INSERT INTO app.search_quota AS q (subject, day, used)
VALUES (@subject, (now() AT TIME ZONE 'UTC')::date, 1)
ON CONFLICT (subject, day) DO UPDATE SET used = q.used + 1
WHERE q.used < @quota_limit::int
RETURNING used;
