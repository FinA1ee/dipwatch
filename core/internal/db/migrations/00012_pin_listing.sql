-- A tracker can follow one listing (offer) instead of the product's cheapest
-- offer. A member still has at most one tracker per product. The limits
-- allow a little more by default: two stores per search, three tracked products.

-- +goose Up
ALTER TABLE app.watch_items
    ADD COLUMN offer_id bigint REFERENCES app.offers (id) ON DELETE SET NULL; -- NULL: the product's cheapest offer

UPDATE app.settings
SET value = value || jsonb_build_object(
        'sources_per_search', GREATEST(COALESCE((value ->> 'sources_per_search')::int, 1), 2),
        'tracked_per_member', GREATEST(COALESCE((value ->> 'tracked_per_member')::int, 1), 3)),
    updated_at = now()
WHERE key = 'limits';

-- +goose Down
ALTER TABLE app.watch_items DROP COLUMN offer_id;
