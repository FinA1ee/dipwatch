# Dipwatch tasks

Daily plan for week 1. Each day has a goal, tasks and success metrics. A day is done when every metric is met and its pull request is merged with green CI. Requirement IDs refer to [spec.md](spec.md).

Detailed tasks are written at the start of each day, so they reflect what the previous day taught us. Status: `[ ]` open, `[x]` done.

## Week overview

| Day | Goal                                    | Success metrics                                                                                                                                                                                                                                               |
| --- | --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Walking skeleton and source feasibility | See Day 1 below                                                                                                                                                                                                                                               |
| 2   | Data layer and first source             | Schema for all tables migrated. River runs a refresh job. `eBay` adapter returns at least 10 offers for "RTX 5090". First deploy to the ECS by CI, reachable at the server IP. Reachability from the ECS recorded for every candidate source and LLM endpoint |
| 3   | Browsing agent                          | Agent strategy returns valid offers from at least 3 sources without an API. Every run recorded in `agent_runs` with tokens and cost. Budgets from C3 enforced by tests                                                                                        |
| 4   | Matching, ranking, search flow          | Filter accuracy at least 90% on the 30-listing eval set (S6). Intent gate and input checks (S3, S4) tested. `POST /search` plus SSE stream works end to end (S1, S2, S7). FX conversion with fallback (R3, R4)                                                |
| 5   | Web app and accounts                    | Search page, ranked results with filters and currency selector, product page. Email magic link and GitHub sign-in work (A1, A2). Quotas C1 enforced                                                                                                           |
| 6   | Tracking and alerts                     | Watch items with notes and rules (T1, T2, C2). Scheduler refreshes tracked products (T3). Simulated price drop delivers an email and a Feishu message within 5 minutes (T4 to T6). Price history chart (T7)                                                   |
| 7   | Hardening and release                   | Spend breaker (C4) tested. One source failing does not break search. Deploy rollback (N3) tested once. README lets a fresh clone run the stack (N4). PWA installable. Retro written                                                                           |

## Day 1: walking skeleton and source feasibility

Goal: every service exists, talks to the next one, runs locally with one command and is checked by CI. We also learn which sources can be fetched and how.

### Success metrics

| #    | Metric                                  | How to check                                                                                                                                                                               |
| ---- | --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| M1.1 | Local stack starts with one command     | `make dev`, then `curl -s localhost:8080/api/healthz` returns `{"status":"ok","db":"ok"}`. The request goes Caddy, gateway, core, Postgres                                                 |
| M1.2 | Web app shows live status               | `http://localhost:8080` shows "core: ok" and "db: ok", fetched with the generated typed client                                                                                             |
| M1.3 | Core is private                         | A request to core without the internal token gets 401 (tested in Go)                                                                                                                       |
| M1.4 | Gateway strips spoofed identity headers | A client-sent `X-Dipwatch-User` never reaches core (tested in TypeScript)                                                                                                                  |
| M1.5 | Contract pipeline works                 | `make contract` regenerates `contracts/openapi.yaml` and the web TypeScript schema. CI fails on drift. The gateway gets generated types on Day 5, when it first calls typed core endpoints |
| M1.6 | CI is green                             | Pull request checks pass: Go lint and tests with Postgres, TypeScript lint, type-check and tests, contract drift, secret scan, image builds                                                |
| M1.7 | Source feasibility known                | [spikes/day1-sources.md](spikes/day1-sources.md) lists at least 10 candidate sources with HTTP result, blocking behavior and starting strategy tier                                        |
| M1.8 | Nothing internal leaks                  | No internal hostnames, company email or registry URLs in the repo or in commit metadata                                                                                                    |

### Tasks

- [x] **1.1 Docs and repo hygiene.** Files: `docs/*`, `README.md`, `.gitignore`, `.npmrc`, `.editorconfig`, `LICENSE`. Done when M1.8 holds for the first commit.
- [x] **1.2 Go core skeleton.** Files: `core/go.mod`, `core/cmd/core-api/main.go`, `core/cmd/worker/main.go`, `core/internal/config`, `core/internal/db` (pgx pool, embedded goose migrations with an `app` schema and `pg_trgm`), `core/internal/http` (huma API, `GET /healthz`, internal-token middleware). Subcommands: `serve`, `migrate`, `openapi`. Tests: health handler reports db status from a fake pinger; middleware rejects missing or wrong token (M1.3); migration test against real Postgres when `DATABASE_URL` is set.
- [x] **1.3 Contract pipeline.** Files: `contracts/openapi.yaml`, `Makefile` target `contract`, `web/src/api/schema.d.ts`. Done when M1.5 holds.
- [x] **1.4 Gateway skeleton.** Files: `gateway/src/app.ts`, `gateway/src/server.ts`, `gateway/src/app.test.ts`. `GET /healthz` for the container. `/api/*` forwarded to core with `X-Dipwatch-Internal` set and all incoming `X-Dipwatch-*` headers removed (M1.4).
- [x] **1.5 Web skeleton.** Files: `web/index.html`, `web/src/main.tsx`, `web/src/App.tsx`, `web/src/api/client.ts`. Tailwind via the Vite plugin. Shows core and db status (M1.2).
- [x] **1.6 Local stack.** Files: `deploy/compose.yaml`, `deploy/Caddyfile`, `core/Dockerfile`, `gateway/Dockerfile`, `web/Dockerfile`, `deploy/.env.example`, `Makefile` target `dev`. Core runs migrations on start. Done when M1.1 holds.
- [x] **1.7 CI.** File: `.github/workflows/ci.yml`. Jobs: `go` (golangci-lint, tests with a Postgres service), `ts` (Biome, `tsc`, Vitest, web build), `contract` (regenerate and diff), `secrets` (gitleaks), `images` (build all three images, no push). Done when M1.6 holds.
- [x] **1.8 Source feasibility spike.** Files: `tools/spike/sources/main.go`, `docs/spikes/day1-sources.md`. One polite GET per source search page for "RTX 5090" from the dev machine. Record status code, response size, whether prices appear in the HTML, and signs of login walls or captchas. Assign a starting tier per source (M1.7). Server-side reachability is Day 2.

### Day 1 notes

- Local host ports are 18080 (site) and 25432 (Postgres) because 8080, 5432 and 55432 were already used on the dev machine. Both are configurable in `deploy/.env`.
- TypeScript is pinned to 6.x. TypeScript 7 is the native compiler and no longer ships the JavaScript API that `openapi-typescript` needs.
- pnpm 12 requires approving dependency build scripts. Only `esbuild` is approved, in `pnpm-workspace.yaml`.
- The spike added an `html` strategy tier between MCP and agent. See [spikes/day1-sources.md](spikes/day1-sources.md).

### Carried to Day 2 (needs input from the owner)

- ECS SSH access stored as GitHub Actions secrets, Docker installed on the ECS, a port open in the security group.
- An Aliyun ACR namespace and push credentials as GitHub Actions secrets.
- eBay developer keys in `deploy/.env` on the server and in a local `.env`.
- One LLM key (DeepSeek or Qwen) in the local `.env`, needed from Day 3.

## Day 2: data layer, job queue, first source, deploy pipeline

Goal: offers can be fetched from a source by a background job and stored with history, and a merge to `main` can deploy to the ECS.

The ECS and eBay keys are not available yet. Everything that needs them is built and tested against fakes now, and verified live once they arrive (marked **live**).

### Success metrics

| #    | Metric                           | How to check                                                                                                                                                                                                                        |
| ---- | -------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| M2.1 | Full schema migrated             | Migration `00002` creates every table in the design's data model. Migrations apply twice without error (Go test, real Postgres)                                                                                                     |
| M2.2 | Typed queries generated          | `sqlc generate` output is committed. CI regenerates it and fails on drift                                                                                                                                                           |
| M2.3 | Strategy fallback works          | Registry tests: first strategy with offers wins; an error falls through to the next; 3 failures in a row mark a strategy unhealthy for 1 hour                                                                                       |
| M2.4 | A job refreshes a product        | Integration test: enqueue `refresh_product` with a fake source; the worker stores offers and one price point per offer; a second run with a new price adds a price point and keeps one offer row                                    |
| M2.5 | First source returns real offers | Kakaku adapter tested against a Shift_JIS fixture. Live: `make source-smoke` returns at least 10 offers for "RTX 5090". (Changed from eBay, see notes)                                                                              |
| M2.6 | Deploy pipeline exists           | `deploy.yml` builds and pushes images to ACR, then deploys over SSH with a health check and rollback to the previous tag. Skips cleanly when secrets are missing. **Live:** merge to `main` is reachable at `http://<ECS_IP>:18080` |
| M2.7 | Server reachability known        | **Live:** the Day 1 spike plus LLM endpoints re-run from the ECS, results added to the spike doc                                                                                                                                    |

### Tasks

- [x] **2.1 Schema.** `core/internal/db/migrations/00002_schema.sql`: `users`, `products`, `queries`, `sources`, `offers`, `price_points`, `watch_items`, `alerts`, `fx_rates`, `agent_runs`, all in schema `app`. Prices are stored as integer minor units (cents, or yen for JPY) plus an ISO currency code, which avoids floating point and a decimal library.
- [x] **2.2 sqlc.** `core/sqlc.yaml`, `core/internal/store/queries/*.sql`, generated code in `core/internal/store`. Makefile target `sqlc`, CI drift check.
- [x] **2.3 Sources package.** `core/internal/sources`: `Offer`, `Query`, `Strategy`, `Source`, `Registry` with ordered fallback and health tracking. Tests for M2.3.
- [x] **2.4 River and the refresh job.** River migrations run with ours. `core/internal/jobs`: `RefreshProductArgs` and its worker, which calls the registry for each enabled source and upserts offers and price points. Worker binary runs River. Integration test for M2.4.
- [x] **2.5 Kakaku adapter.** `core/internal/sources/kakaku`: html strategy with goquery, Shift_JIS decoding, catalog and mall item IDs. Fixture tests (M2.5). `cmd/source-smoke` for the live check.
- [ ] **2.6 Deploy pipeline (live part open).** `.github/workflows/deploy.yml`, `deploy/deploy.sh` (pull, up, health check, rollback). `deploy/compose.yaml` reads images from ACR via `IMAGE_PREFIX`. Written and passing `actionlint`; the workflow skips until the ECS and ACR secrets exist.
- [ ] **2.7 Server spike (live).** Run the spike and an LLM reachability check on the ECS.

### Day 2 notes

- **eBay replaced by Kakaku** as the first source. eBay needs a developer keyset and its listings are mostly auctions and resellers; Kakaku needed no keys and a live search returned 44 offers.
- **Price points are written only on change.** An unchanged re-check updates `last_seen_at` and adds no row, so history stays small.
- **River uniqueness:** a refresh job is unique per product only while it is waiting or running. River's default also counts completed jobs, which would have blocked every later refresh of the product.
- **Postgres on the server** comes from a copy in ACR (`dipwatch-postgres:16`), pushed by the deploy workflow, because the server cannot pull from Docker Hub.
- Verified end to end in the local stack: `make refresh Q="RTX 5090"` stored 44 offers and 44 price points from Kakaku.
