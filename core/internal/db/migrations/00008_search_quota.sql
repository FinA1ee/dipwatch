-- Search quotas (spec C1): costly searches per subject and day. A subject is
-- "ip:<address>" for visitors or "user:<id>" for members. Days are UTC.

-- +goose Up
CREATE TABLE app.search_quota (
    subject  text NOT NULL,
    day      date NOT NULL,
    used     int NOT NULL DEFAULT 0,
    PRIMARY KEY (subject, day)
);

-- +goose Down
DROP TABLE app.search_quota;
