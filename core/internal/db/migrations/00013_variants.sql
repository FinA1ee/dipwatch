-- Variants and price trust (week 2). A listing's variant is the spec that sets
-- its price within one model, such as "24GB" or "48GB" for an RTX 4090; the
-- matcher extracts it. A tracker can follow one variant. A price is
-- unconfirmed when the store only relays it (Kakaku's links to shop feeds)
-- rather than showing it on a page it compares.

-- +goose Up
ALTER TABLE app.match_cache ADD COLUMN variant text NOT NULL DEFAULT '';
-- Cached decisions have no variant: let the next refresh match every listing
-- again (one cheap call per product and store).
DELETE FROM app.match_cache;

ALTER TABLE app.offers
    ADD COLUMN variant text NOT NULL DEFAULT '',                -- '' until matched, or when the title does not say
    ADD COLUMN price_unconfirmed boolean NOT NULL DEFAULT false;

ALTER TABLE app.watch_items ADD COLUMN variant text NOT NULL DEFAULT ''; -- '': any variant

-- +goose Down
ALTER TABLE app.watch_items DROP COLUMN variant;
ALTER TABLE app.offers DROP COLUMN price_unconfirmed, DROP COLUMN variant;
ALTER TABLE app.match_cache DROP COLUMN variant;
