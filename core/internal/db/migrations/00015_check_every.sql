-- A member can choose how often a tracked product is re-checked. NULL keeps
-- the automatic interval (6 hours, between 2 and 24 hours as prices move or
-- stay still). The product is refreshed at the shortest interval any of its
-- trackers asks for; the owner's limit min_check_hours bounds it.

-- +goose Up
ALTER TABLE app.watch_items ADD COLUMN check_every interval; -- NULL: automatic

-- +goose Down
ALTER TABLE app.watch_items DROP COLUMN check_every;
