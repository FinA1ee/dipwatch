-- Query gate (spec S3, S4): every normalized query text is cached with its intent
-- verdict, so a spelling is sent to the LLM once. A null product_id means the
-- intent check decided the text is not a product.

-- +goose Up
ALTER TABLE app.queries ALTER COLUMN product_id DROP NOT NULL;

-- +goose Down
DELETE FROM app.queries WHERE product_id IS NULL;
ALTER TABLE app.queries ALTER COLUMN product_id SET NOT NULL;
