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
