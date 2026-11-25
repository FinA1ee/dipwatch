-- LLM matching (spec S6): whether a listing is the product itself, plus an English
-- title and condition. Decisions are cached per product and listing title, so an
-- unchanged listing is never sent to the LLM twice.

-- +goose Up
ALTER TABLE app.offers ADD COLUMN is_product boolean; -- null until matched

CREATE TABLE app.match_cache (
    product_id  bigint NOT NULL REFERENCES app.products (id) ON DELETE CASCADE,
    title_hash  text NOT NULL, -- sha256 of the normalized listing title
    is_product  boolean NOT NULL,
    title_en    text NOT NULL,
    condition   text NOT NULL DEFAULT 'unknown'
                CHECK (condition IN ('new', 'used', 'refurbished', 'unknown')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (product_id, title_hash)
);

-- +goose Down
DROP TABLE app.match_cache;
ALTER TABLE app.offers DROP COLUMN is_product;
