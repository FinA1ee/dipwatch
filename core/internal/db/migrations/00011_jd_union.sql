-- JD through JD Union (task 7.3): its search page shows servers a login wall, so
-- the store uses the affiliate API. It stays disabled until the owner has an
-- approved app key; then: UPDATE app.sources SET enabled = true WHERE id = 'jd';

-- +goose Up
UPDATE app.sources SET strategies = '{api}', name = 'JD' WHERE id = 'jd';

-- +goose Down
UPDATE app.sources SET strategies = '{agent}' WHERE id = 'jd';
