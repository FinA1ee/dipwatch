-- Full week-1 data model. See docs/design.md "Data model".
-- Money is stored as integer minor units (cents; yen for JPY) plus an ISO 4217 code.

-- +goose Up
CREATE TABLE app.users (
    id          text PRIMARY KEY, -- Better Auth user id
    email       text NOT NULL UNIQUE,
    currency    char(3) NOT NULL DEFAULT 'CNY',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.products (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    canonical_name      text NOT NULL UNIQUE,
    category            text NOT NULL DEFAULT '',
    aliases             text[] NOT NULL DEFAULT '{}',
    last_refreshed_at   timestamptz,
    last_discovered_at  timestamptz,
    -- Adaptive re-check interval (design: Refresh policy and cost).
    refresh_interval    interval NOT NULL DEFAULT '6 hours',
    unchanged_checks    int NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX products_canonical_name_trgm ON app.products USING gin (canonical_name gin_trgm_ops);

CREATE TABLE app.queries (
    normalized_text  text PRIMARY KEY,
    product_id       bigint NOT NULL REFERENCES app.products (id) ON DELETE CASCADE,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.sources (
    id          text PRIMARY KEY, -- slug, e.g. ebay-us
    name        text NOT NULL,
    region      text NOT NULL,
    currency    char(3) NOT NULL,
    strategies  text[] NOT NULL DEFAULT '{}', -- ordered: api, mcp, html, agent
    enabled     boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.offers (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id      bigint NOT NULL REFERENCES app.products (id) ON DELETE CASCADE,
    source_id       text NOT NULL REFERENCES app.sources (id),
    external_id     text NOT NULL,
    url             text NOT NULL,
    title_original  text NOT NULL,
    title_en        text NOT NULL DEFAULT '',
    price_minor     bigint NOT NULL CHECK (price_minor >= 0),
    currency        char(3) NOT NULL,
    shipping_minor  bigint CHECK (shipping_minor >= 0),
    condition       text NOT NULL DEFAULT 'unknown'
                    CHECK (condition IN ('new', 'used', 'refurbished', 'unknown')),
    availability    text NOT NULL DEFAULT 'unknown'
                    CHECK (availability IN ('in_stock', 'out_of_stock', 'unknown')),
    seller_type     text NOT NULL DEFAULT 'unknown'
                    CHECK (seller_type IN ('official', 'third_party', 'individual', 'unknown')),
    specs           jsonb NOT NULL DEFAULT '{}',
    raw             jsonb,
    first_seen_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_id, external_id)
);
CREATE INDEX offers_product_id ON app.offers (product_id);

CREATE TABLE app.price_points (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    offer_id     bigint NOT NULL REFERENCES app.offers (id) ON DELETE CASCADE,
    observed_at  timestamptz NOT NULL DEFAULT now(),
    price_minor  bigint NOT NULL CHECK (price_minor >= 0),
    currency     char(3) NOT NULL,
    in_stock     boolean
);
CREATE INDEX price_points_offer_observed ON app.price_points (offer_id, observed_at DESC);

CREATE TABLE app.watch_items (
    id                        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id                   text NOT NULL REFERENCES app.users (id) ON DELETE CASCADE,
    product_id                bigint NOT NULL REFERENCES app.products (id) ON DELETE CASCADE,
    note                      varchar(500) NOT NULL DEFAULT '',
    rules                     jsonb NOT NULL DEFAULT '[]',
    channels                  jsonb NOT NULL DEFAULT '[]',
    baseline_price_cny_minor  bigint,
    created_at                timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, product_id)
);
CREATE INDEX watch_items_product_id ON app.watch_items (product_id);

CREATE TABLE app.alerts (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    watch_item_id  bigint NOT NULL REFERENCES app.watch_items (id) ON DELETE CASCADE,
    offer_id       bigint NOT NULL REFERENCES app.offers (id) ON DELETE CASCADE,
    rule           text NOT NULL,
    channel        text NOT NULL,
    alert_date     date NOT NULL DEFAULT current_date,
    status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    sent_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    -- At most one alert per offer, rule and channel per day (spec T5).
    UNIQUE (watch_item_id, offer_id, rule, channel, alert_date)
);

CREATE TABLE app.fx_rates (
    date   date NOT NULL,
    base   char(3) NOT NULL,
    rates  jsonb NOT NULL,
    PRIMARY KEY (date, base)
);

CREATE TABLE app.agent_runs (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind              text NOT NULL CHECK (kind IN ('intent', 'match', 'browse')),
    source_id         text REFERENCES app.sources (id),
    product_id        bigint REFERENCES app.products (id) ON DELETE SET NULL,
    model             text NOT NULL,
    steps             int NOT NULL DEFAULT 0,
    tokens_in         int NOT NULL DEFAULT 0,
    tokens_out        int NOT NULL DEFAULT 0,
    cost_cny_micros   bigint NOT NULL DEFAULT 0, -- millionths of a yuan
    duration_ms       int NOT NULL DEFAULT 0,
    outcome           text NOT NULL,
    trace             jsonb,
    created_at        timestamptz NOT NULL DEFAULT now()
);
-- The spend breaker (spec C4) sums cost over the current day.
CREATE INDEX agent_runs_created_at ON app.agent_runs (created_at);

-- +goose Down
DROP TABLE app.agent_runs, app.fx_rates, app.alerts, app.watch_items, app.price_points,
    app.offers, app.sources, app.queries, app.products, app.users;
