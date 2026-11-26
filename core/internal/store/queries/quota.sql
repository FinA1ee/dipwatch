-- name: ChargeQuota :one
-- Counts one costly search for the subject today, unless the limit is reached.
-- No row returned means the subject is over its limit.
INSERT INTO app.search_quota AS q (subject, day, used)
VALUES (@subject, (now() AT TIME ZONE 'UTC')::date, 1)
ON CONFLICT (subject, day) DO UPDATE SET used = q.used + 1
WHERE q.used < @quota_limit::int
RETURNING used;

-- name: DeleteOldQuota :execrows
-- Search counters name a visitor's IP address: kept for 30 days (privacy page).
DELETE FROM app.search_quota WHERE day < (now() AT TIME ZONE 'UTC')::date - 30;

-- name: DeleteOldSessions :execrows
-- Sign-in sessions hold an IP address and browser name: removed 30 days after they expire.
DELETE FROM auth.session WHERE "expiresAt" < now() - interval '30 days';
