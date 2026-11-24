-- name: ListEnabledSources :many
SELECT * FROM app.sources WHERE enabled ORDER BY id;

-- name: UpsertSource :exec
INSERT INTO app.sources (id, name, region, currency, strategies, enabled)
VALUES (@id, @name, @region, @currency, @strategies, @enabled)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    region = EXCLUDED.region,
    currency = EXCLUDED.currency,
    strategies = EXCLUDED.strategies,
    enabled = EXCLUDED.enabled;
