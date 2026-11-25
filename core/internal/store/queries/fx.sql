-- name: UpsertFXRates :exec
INSERT INTO app.fx_rates (date, base, rates)
VALUES (@date, @base, @rates)
ON CONFLICT (date, base) DO UPDATE SET rates = EXCLUDED.rates;

-- name: LatestFXRates :one
-- The most recent stored rates, so conversion keeps working when the API is down (spec R4).
SELECT * FROM app.fx_rates WHERE base = @base ORDER BY date DESC LIMIT 1;
