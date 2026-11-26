-- Rule "change": alert when the tracked price moves by a percentage, up or
-- down. A tracker remembers the last price it saw; an alert records the price
-- before and after, for the message.

-- +goose Up
ALTER TABLE app.watch_items ADD COLUMN last_price_cny_minor bigint; -- NULL until a price is seen
UPDATE app.watch_items SET last_price_cny_minor = baseline_price_cny_minor;

ALTER TABLE app.alerts
    ADD COLUMN previous_cny_minor bigint, -- rule "change" only
    ADD COLUMN current_cny_minor bigint;

-- +goose Down
ALTER TABLE app.alerts DROP COLUMN current_cny_minor, DROP COLUMN previous_cny_minor;
ALTER TABLE app.watch_items DROP COLUMN last_price_cny_minor;
