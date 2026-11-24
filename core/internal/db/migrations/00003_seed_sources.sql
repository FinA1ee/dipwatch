-- Candidate sources from docs/spikes/day1-sources.md. A source runs only when it is
-- enabled here AND an adapter for its strategies is registered in the worker.

-- +goose Up
INSERT INTO app.sources (id, name, region, currency, strategies, enabled) VALUES
    ('ebay-us',      'eBay',            'US', 'USD', '{api}',        false),
    ('kakaku',       'Kakaku.com',      'JP', 'JPY', '{html}',       true),
    ('amazon-jp',    'Amazon Japan',    'JP', 'JPY', '{html,agent}', false),
    ('amazon-us',    'Amazon US',       'US', 'USD', '{html,agent}', false),
    ('jd',           'JD.com',          'CN', 'CNY', '{agent}',      false),
    ('smzdm',        'SMZDM',           'CN', 'CNY', '{agent}',      false),
    ('newegg',       'Newegg',          'US', 'USD', '{agent}',      false),
    ('bhphoto',      'B&H Photo',       'US', 'USD', '{agent}',      false),
    ('pcpartpicker', 'PCPartPicker',    'US', 'USD', '{agent}',      false)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM app.sources WHERE id IN
    ('ebay-us', 'kakaku', 'amazon-jp', 'amazon-us', 'jd', 'smzdm', 'newegg', 'bhphoto', 'pcpartpicker');
