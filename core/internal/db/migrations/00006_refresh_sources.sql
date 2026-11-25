-- Per-source refresh (spec S2): a product refresh fans out to one job per source.
-- refreshing_sources lists the sources still running, so the search stream knows
-- when the last one has finished.

-- +goose Up
ALTER TABLE app.products ADD COLUMN refreshing_sources text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE app.products DROP COLUMN refreshing_sources;
