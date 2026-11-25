-- Tracking and alerts (spec T1–T6). Alert channels belong to the member, not to
-- each tracked product, so they live on app.users; watch_items.channels stays
-- unused. Refresh bookkeeping for the adaptive interval (T3) lives on products.

-- +goose Up
ALTER TABLE app.users
    ADD COLUMN feishu_webhook text NOT NULL DEFAULT '', -- empty: no Feishu alerts
    ADD COLUMN email_alerts boolean NOT NULL DEFAULT true;

ALTER TABLE app.products
    ADD COLUMN refresh_changed boolean NOT NULL DEFAULT false; -- a known offer's price or stock changed in the current refresh

-- +goose Down
ALTER TABLE app.products DROP COLUMN refresh_changed;
ALTER TABLE app.users DROP COLUMN email_alerts, DROP COLUMN feishu_webhook;
