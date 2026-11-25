-- Budget limits (task 6.7): limits the owner changes at runtime, and freshness
-- per product and store, since a search now refreshes only the stores it names.

-- +goose Up
CREATE TABLE app.settings (
    key         text PRIMARY KEY,
    value       jsonb NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.product_sources (
    product_id         bigint NOT NULL REFERENCES app.products (id) ON DELETE CASCADE,
    source_id          text NOT NULL REFERENCES app.sources (id) ON DELETE CASCADE,
    last_refreshed_at  timestamptz NOT NULL,
    PRIMARY KEY (product_id, source_id)
);

-- The stores a tracker refreshes and alerts on. Empty (rows from before this
-- change) means every enabled store.
ALTER TABLE app.watch_items ADD COLUMN sources text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE app.watch_items DROP COLUMN sources;
DROP TABLE app.product_sources;
DROP TABLE app.settings;
