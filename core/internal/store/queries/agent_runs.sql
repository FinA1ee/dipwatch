-- name: InsertAgentRun :one
INSERT INTO app.agent_runs (
    kind, source_id, product_id, model, steps, tokens_in, tokens_out,
    cost_cny_micros, duration_ms, outcome, trace
) VALUES (
    @kind, sqlc.narg('source_id'), sqlc.narg('product_id'), @model, @steps, @tokens_in, @tokens_out,
    @cost_cny_micros, @duration_ms, @outcome, @trace
)
RETURNING id;

-- name: SpendTodayMicros :one
-- Used by the daily spend breaker (spec C4). "Today" is in Beijing time.
SELECT coalesce(sum(cost_cny_micros), 0)::bigint
FROM app.agent_runs
WHERE created_at >= date_trunc('day', now() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai';

-- name: GetAgentRun :one
SELECT * FROM app.agent_runs WHERE id = @id;

-- name: SourceRunMetrics :many
-- Per-source browsing-agent metrics per UTC day (spec H3). Success means the
-- run answered: offers, or a confirmed "no listings".
SELECT r.source_id::text AS source_id,
       (r.created_at AT TIME ZONE 'UTC')::date AS day,
       count(*)::int AS runs,
       (count(*) FILTER (WHERE r.outcome IN ('ok', 'no_offers')))::int AS answered,
       coalesce(avg((r.trace->>'offers')::int), 0)::float8 AS offers_per_run,
       (percentile_cont(0.5) WITHIN GROUP (ORDER BY r.steps))::float8 AS median_steps,
       avg(r.tokens_in + r.tokens_out)::float8 AS tokens_per_run,
       avg(r.cost_cny_micros)::float8 AS cost_micros_per_run,
       coalesce(mode() WITHIN GROUP (ORDER BY r.outcome) FILTER (WHERE r.outcome NOT IN ('ok', 'no_offers')), '')::text AS top_failure
FROM app.agent_runs r
WHERE r.kind = 'browse' AND r.source_id IS NOT NULL AND r.created_at >= @since
GROUP BY 1, 2
ORDER BY 2 DESC, 1;

-- name: SpendByKind :many
-- LLM spend per kind (browse, match, intent) per UTC day.
SELECT kind, (created_at AT TIME ZONE 'UTC')::date AS day, count(*)::int AS calls,
       sum(cost_cny_micros)::bigint AS cost_micros
FROM app.agent_runs
WHERE created_at >= @since
GROUP BY 1, 2
ORDER BY 2 DESC, 1;

-- name: ListAgentRuns :many
-- Run list for the admin page (spec H4), newest first; filters are optional.
SELECT id, kind, source_id, product_id, model, steps, tokens_in, tokens_out, cost_cny_micros,
       duration_ms, outcome, coalesce(trace->>'prompt_version', '')::text AS prompt_version,
       coalesce((trace->>'offers')::int, 0)::int AS offers, created_at
FROM app.agent_runs
WHERE (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind')::text)
  AND (sqlc.narg('source_id')::text IS NULL OR source_id = sqlc.narg('source_id')::text)
  AND (sqlc.narg('outcome')::text IS NULL OR outcome = sqlc.narg('outcome')::text)
  AND (sqlc.narg('prompt_version')::text IS NULL OR trace->>'prompt_version' = sqlc.narg('prompt_version')::text)
  AND (sqlc.narg('day')::date IS NULL OR (created_at AT TIME ZONE 'UTC')::date = sqlc.narg('day')::date)
  AND (sqlc.narg('before_id')::bigint IS NULL OR id < sqlc.narg('before_id')::bigint)
ORDER BY id DESC
LIMIT @row_limit::int;

