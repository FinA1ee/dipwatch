# Dipwatch tasks

Daily plan for week 1. Each day has a goal, tasks and success metrics. A day is done when every metric is met and its pull request is merged with green CI. Requirement IDs refer to [spec.md](spec.md).

Detailed tasks are written at the start of each day, so they reflect what the previous day taught us. Status: `[ ]` open, `[x]` done.

## Week overview

| Day | Goal                                      | Success metrics                                                                                                                                                                                                                                                                                                                                                 |
| --- | ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Walking skeleton and source feasibility   | See Day 1 below                                                                                                                                                                                                                                                                                                                                                 |
| 2   | Data layer and first source               | Schema for all tables migrated. River runs a refresh job. `eBay` adapter returns at least 10 offers for "RTX 5090". First deploy to the ECS by CI, reachable at the server IP. Reachability from the ECS recorded for every candidate source and LLM endpoint                                                                                                   |
| 3   | Browsing agent                            | Agent strategy returns valid offers from at least 3 sources without an API. Every run recorded in `agent_runs` with tokens and cost. Budgets from C3 enforced by tests                                                                                                                                                                                          |
| 4   | Agent eval harness, matching, search flow | Eval set of about 10 saved pages with labels; `make agent-eval` reports precision, recall, steps and cost per store (H1). Agent precision at least 90%, recall at least 60%. Matcher filter accuracy at least 90% on 30 listings (S6). Intent gate and input checks (S3, S4). `POST /search` plus SSE stream (S1, S2, S7). FX conversion with fallback (R3, R4) |
| 5   | Web app, accounts, run viewer             | Search page, ranked results with filters and currency selector, product page. Email magic link and GitHub sign-in (A1, A2). Quotas C1. Admin run viewer with per-source metrics and trace view (H3, H4)                                                                                                                                                         |
| 6   | Tracking and alerts                       | Watch items with notes and rules (T1, T2, C2). Scheduler refreshes tracked products (T3). Simulated price drop delivers an email within 5 minutes (T4, T5). Price history chart (T7)                                                                                                                                                                            |
| 7   | Hardening, guardrails, release            | Spend breaker (C4) stops agent runs, tested. Prompt-injection eval case passes (H5). Two prompt versions compared on the eval set and recorded (H6). One source failing does not break search. Deploy rollback (N3) tested once. Database backup. README lets a fresh clone run the stack (N4). Retro written                                                   |

Moved out of week 1 on Day 4 to make room for the agent harness (spec H1 to H6): the installable PWA and Feishu alerts. Email stays the week-1 alert channel.

## Day 1: walking skeleton and source feasibility

Goal: every service exists, talks to the next one, runs locally with one command and is checked by CI. We also learn which sources can be fetched and how.

### Success metrics

| #    | Metric                                  | How to check                                                                                                                                                                               |
| ---- | --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| M1.1 | Local stack starts with one command     | `make dev`, then `curl -s localhost:18080/api/healthz` returns `{"status":"ok","db":"ok"}`. The request goes Caddy, gateway, core, Postgres                                                |
| M1.2 | Web app shows live status               | `http://localhost:18080` shows "core: ok" and "db: ok", fetched with the generated typed client                                                                                            |
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
- [x] **2.6 Deploy pipeline.** `.github/workflows/deploy.yml`, `deploy/deploy.sh` (pull, up, health check, rollback). `deploy/compose.yaml` reads images from one ACR repository via `IMAGE_REPO`, with tags like `core-<sha>`. Written and passing `actionlint`; the workflow skips until the ECS and ACR secrets exist.
- [x] **2.7 Server spike (live).** Run the spike and an LLM reachability check on the ECS.

### Day 2 notes

- **eBay replaced by Kakaku** as the first source. eBay needs a developer keyset and its listings are mostly auctions and resellers; Kakaku needed no keys and a live search returned 44 offers.
- **Price points are written only on change.** An unchanged re-check updates `last_seen_at` and adds no row, so history stays small.
- **River uniqueness:** a refresh job is unique per product only while it is waiting or running. River's default also counts completed jobs, which would have blocked every later refresh of the product.
- **ACR address:** newer ACR Personal Edition instances have their own domain (`<instance>.cn-hangzhou.personal.cr.aliyuncs.com`), shown on the console's 访问凭证 page. The shared `registry.cn-hangzhou.aliyuncs.com` returned 403 for this instance. All images live in one repository, `dipwatch`, with tags `core-<sha>`, `gateway-<sha>`, `web-<sha>` and `postgres-16`.
- **Postgres on the server** comes from a copy in ACR (`dipwatch-postgres:16`), pushed by the deploy workflow, because the server cannot pull from Docker Hub.
- Verified end to end in the local stack: `make refresh Q="RTX 5090"` stored 44 offers and 44 price points from Kakaku.
- **Server checked (2.7):** Kakaku and Amazon US work from the ECS; Amazon Japan times out there. DeepSeek, Qwen and Kimi are reachable; OpenAI, Google OAuth, Docker Hub and the Go proxy are not. Google sign-in is dropped.
- **First live deploy (2.6):** merge of `be35549` to `main` passed CI, deployed itself, and the health URL answers from the internet on the server IP. Fixes needed on the way: the personal ACR domain, buildx attestations off for ACR, and running `deploy.sh` with `sh` because `scp` drops the executable bit.

## Day 3: browsing agent

Goal: an `agent` strategy that drives a real browser with an LLM and returns validated offers, with every run budgeted and recorded.

The LLM key is not available yet. The agent is built and tested against a scripted fake model and a fake browser; the live run against real stores happens once the key arrives (marked **live**).

### Success metrics

| #    | Metric                                                  | How to check                                                                                                                                                                               |
| ---- | ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| M3.1 | Provider-neutral LLM client                             | `internal/llm` talks to any OpenAI-compatible endpoint, set by env. Tested against a fake server: tool calls and token usage parsed, cost computed from configured prices                  |
| M3.2 | Agent loop works                                        | Test with a scripted model and fake browser: navigate, snapshot, `submit_offers`, offers returned                                                                                          |
| M3.3 | Budgets enforced (C3)                                   | Tests: run stops at 15 steps, 30,000 tokens and 90 seconds, each with its own outcome                                                                                                      |
| M3.4 | Agent stays on the store                                | Tests: navigation to another host is refused without calling the browser; tools outside the allowlist are refused; offers whose URL is off the store are dropped                           |
| M3.5 | Output is validated                                     | Tests: invalid prices or missing fields are dropped; a submission with no valid offers is sent back to the model once; a "blocked" report fails the strategy so the registry can fall back |
| M3.6 | Every run recorded (C5)                                 | Each run writes one `agent_runs` row with model, steps, tokens, cost and outcome (integration test)                                                                                        |
| M3.7 | Real browser wired up                                   | The worker image contains Node, Chromium and Playwright MCP. A Go test talks to an in-memory MCP server through the same client code                                                       |
| M3.8 | **Live:** offers from at least 3 sources without an API | `make source-smoke SOURCE=<id>` with a real key returns valid offers for 3 of: Amazon US, Amazon Japan, SMZDM (the stores a real browser can read, see notes)                              |

### Tasks

- [x] **3.1 LLM client.** `core/internal/llm`: `Model` interface with provider-neutral `Message`, `Tool`, `ToolCall` and `Turn` types; `OpenAI` implementation on `openai-go`; config from `LLM_BASE_URL`, `LLM_MODEL`, `LLM_API_KEY`, `LLM_PRICE_IN_CNY_PER_M`, `LLM_PRICE_OUT_CNY_PER_M`.
- [x] **3.2 Browser client.** `core/internal/agent/browser.go`: start Playwright MCP as a child process, list and call tools through the Go MCP SDK.
- [x] **3.3 Agent loop.** `core/internal/agent/agent.go`: system prompt, tool allowlist, host check, `submit_offers` validation, budgets, result truncation, trace.
- [x] **3.4 Run recording.** `InsertAgentRun` query and a recorder used by every run.
- [x] **3.5 Agent strategy.** `sources.Strategy` named `agent`, one task template per store (search URL, allowed hosts, currency).
- [x] **3.6 Worker image.** Separate `worker` image with Node 24, Chromium and Playwright MCP; compose, CI and deploy updated.
- [x] **3.7 Live check (live).** Run the agent against the five candidate stores and enable the ones that work.

### Day 3 notes

- **Real browser check (no LLM):** `source-smoke -browser-only` opened each store in headless Chromium, on the Mac and inside the worker image. Amazon US (served as Amazon Singapore to this location), Amazon Japan and SMZDM show results. Newegg, B&H and PCPartPicker show a bot-check page (HTTP 403), and JD redirects to its verification page. The live target therefore uses the first three.
- **Pages are too big for the model as-is.** Amazon's snapshot is about 220,000 characters. `compact` (drop refs on non-clickable lines, display attributes, query strings, repeated links, empty containers, price fragments) and `excerpt` (keep a window after each line naming the product) bring it to 12,000 characters with 8 to 13 listings.
- **Currencies:** stores show the visitor's local currency (Amazon US showed SGD), so the agent records the currency on the page and only checks that it is an ISO code. Conversion happens in the matcher (Day 4).
- **Model:** DeepSeek `deepseek-flash`, the cheapest current model, with thinking turned off via `LLM_EXTRA_BODY`. Prices configured at the peak rate (2 and 8 yuan per million input and output tokens) so cost estimates are on the high side.
- **Prompt version** is recorded in every run's trace (`browse-v1`), so later prompt changes can be compared.
- **Worker image** is a separate build target with Node 24, Chromium and Playwright MCP 0.0.86, about 1.9 GB. Compose gives it 512 MB of shared memory, which Chromium needs.

### Day 3 live results (M3.8 met)

DeepSeek `deepseek-flash`, query "RTX 5090", limit 10, from the dev machine:

| Store        | Outcome | Offers | Steps | Tokens in / out | Cost   |
| ------------ | ------- | ------ | ----- | --------------- | ------ |
| Amazon US    | ok      | 1      | 6     | 25,424 / 588    | ¥0.056 |
| Amazon Japan | ok      | 1      | 6     | 28,672 / 1,382  | ¥0.068 |
| SMZDM        | ok      | 5      | 5     | 21,985 / 760    | ¥0.050 |

Open quality issues for the Day 4 eval: Amazon runs return only one offer although the page shows 9 to 13; SMZDM prices may be list prices rather than deal prices; titles in Chinese still need English titles (matcher).

### How the harness got there (prompt and harness versions)

Each version was one live run on Amazon US. This is the kind of iteration the Day 4 eval set will make measurable.

| Version   | Change                                                                                                                                                                                      | Result                                                                                                        |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| browse-v1 | First version                                                                                                                                                                               | Token limit after 9 steps: tried to switch from amazon.sg back to amazon.com, then searched with browser_find |
| browse-v2 | Trim old tool results from the history (each call re-sends the whole conversation); tell the model not to change region; submit early                                                       | Token limit after 8 steps: browser_find returned 40 matches each time, tried page 2                           |
| browse-v3 | Cap browser_find at 15 matches; tool results 8,000 characters; one results page only; allow amazon.sg                                                                                       | Token limit after 9 steps: still searching                                                                    |
| browse-v4 | Found the root cause: Playwright MCP answers navigate with a link to a snapshot file, not the page. The harness now takes a snapshot after every page-changing tool and returns the excerpt | Token limit: page visible, but the model kept searching and the page was trimmed out of the history           |
| browse-v5 | Keep the latest page in full while trimming older results; at 70% of the step or token budget, offer only submit_offers and report_blocked                                                  | **ok** on all three stores, 5 to 6 steps, about ¥0.05 to ¥0.07 per run                                        |

## Day 4: agent eval harness, matching, search flow

Goal: the agent's quality becomes a number, and a user query becomes ranked offers.

Order matters: the eval harness comes first, so every change after it (prompt, matcher) is measured instead of judged by eye.

### Success metrics

| #    | Metric                                | How to check                                                                                                                                                                                 |
| ---- | ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| M4.1 | Eval set exists                       | About 10 cases under `core/internal/agent/eval/testdata`: saved real results pages (Amazon US, Amazon Japan, SMZDM; several products) with labelled listings, plus one prompt-injection page |
| M4.2 | Eval runs the real loop               | `make agent-eval` replays each page through the unchanged agent with the real LLM and prints precision, recall, steps, tokens and cost per case, and writes a report to `docs/harness/`      |
| M4.3 | Eval is tested without spending money | CI runs the replay browser and scoring with a scripted model                                                                                                                                 |
| M4.4 | Agent quality baseline and target     | Baseline for browse-v5 recorded in `docs/harness.md`. Target precision at least 90%, recall at least 60% (H1); improvements recorded as new versions (H6)                                    |
| M4.5 | Injection page is safe                | The injection case never leaves the store or submits injected text (H5)                                                                                                                      |
| M4.6 | FX conversion                         | Daily rates stored in `fx_rates`; prices converted to CNY; the last stored rates are used when the API is down (R3, R4)                                                                      |
| M4.7 | Matcher                               | One LLM call per batch decides whether each listing is the product, adds an English title and condition; results cached by listing hash. Accuracy at least 90% on 30 labelled listings (S6)  |
| M4.8 | Query gate                            | Input checks (S3) and an intent check that maps spellings to one canonical product, cached in `queries` (S4)                                                                                 |
| M4.9 | Search API                            | `POST /search` returns cached offers or starts per-source refresh jobs; an SSE endpoint streams offers as sources finish (S1, S2, S7)                                                        |

### Tasks

- [x] **4.1 Eval cases.** Capture raw snapshots with `source-smoke -dump`, store them gzipped, label listings with a draft script plus manual review.
- [x] **4.2 Replay browser and scoring.** `core/internal/agent/eval`: a `Browser` that serves a saved page (navigate, snapshot, find), case loading, offer matching, precision and recall.
- [x] **4.3 Eval command.** `core/cmd/agent-eval` and `make agent-eval`; report table and markdown file.
- [x] **4.4 Baseline and first improvement.** Run the eval for browse-v5, record it, improve recall, record again.
- [x] **4.5 FX.** `internal/fx`: fetch from `api.frankfurter.dev`, store daily, convert with fallback. River periodic job every 6 hours.
- [x] **4.6 Matcher.** `internal/matcher`: batch LLM call with a JSON schema, cache, eval on 30 listings. Runs after each refresh; decisions cached in `match_cache`; each call recorded in `agent_runs` (kind `match`). Eval: 98.9% accuracy on 283 listings, ¥0.14 ([report](harness/2026-11-25-1110-match-v1.md)).
- [x] **4.7 Query gate.** Input checks, normalization, intent LLM call, `queries` cache. `internal/guard`: NFKC and lower case make the cache key; rejections are cached too; the intent call sees existing products that share a model number, so spellings converge. Eval: 6 of 6 spelling groups map to one name, 7 of 7 non-products rejected, ¥0.05 ([report](harness/2026-11-25-1114-intent-v1.md)).
- [x] **4.8 Search API and SSE.** Per-source refresh jobs, `POST /search`, `GET /search/{id}/events`, contract and web types regenerated. The id is the product id. `refresh_product` now fans out one `refresh_source` job per source; each records itself finished in `products.refreshing_sources` and sends a Postgres NOTIFY that core-api turns into an SSE event carrying the full ranked list. Offers are ranked by their CNY value and converted to the requested currency.

### Day 4 results

- All metrics M4.1 to M4.9 met. Local end-to-end check: `rtx4090显卡` resolved to "NVIDIA GeForce RTX 4090", SMZDM's offers arrived on the stream after 14 seconds, matched and converted to USD, and the matcher dropped a 48 GB server card.
- Bug found by the end-to-end check: the worker image had no CA certificates, so every HTTPS call from the worker failed, in production too since Day 3. Fixed in `core/Dockerfile`.
- A refresh where every source fails no longer marks the product fresh, so the next search retries instead of showing nothing for 6 hours.
- Open: the agent reports Amazon US "no offers" for RTX 4090 (it found none on the first page); `no_offers` still counts as a strategy failure in the registry. 4090D listings count as the RTX 4090 (decided 2026-11-25; browse-v8 and match-v2).

## Day 5: accounts, quotas, web app, run viewer

Goal: a visitor can search on the website and watch results arrive, sign in by email or GitHub, and the owner can see what every agent run did and cost.

Decisions taken before starting (2026-11-25): web pages use clean architecture (design: Web app structure); the site address is the server IP and port until there is a domain; magic links go out through the owner's mailbox over SMTP; the three agent sources (Amazon US, Amazon Japan, SMZDM) are enabled in production only after quotas are deployed.

### Success metrics

| #     | Metric                           | How to check                                                                                                                                                    |
| ----- | -------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| M5.1  | Search page                      | Typing a product shows stored offers at once, then each source's offers as it finishes, ranked by price, with source, region, original and converted price (S5) |
| M5.2  | Filters and currency             | Results can be filtered by condition, region and stock, and shown in CNY, USD, JPY, EUR or SGD; the choice survives a reload                                    |
| M5.3  | Product page                     | `/p/<id>` lists all offers and the lowest price per day for the last 30 days                                                                                    |
| M5.4  | Sign-in                          | Magic link and GitHub sign-in both work against the deployed site; the same verified email by both methods is one member (A1, A2)                               |
| M5.5  | Quotas                           | Tests: the 11th costly search from one IP in a day and the 31st from one member get 429; cached searches are never counted (C1)                                 |
| M5.6  | Identity headers are trusted     | Tests: a client-sent `X-Dipwatch-*` header never reaches core; admin endpoints answer 403 without the admin role                                                |
| M5.7  | Per-source metrics               | `/admin` shows, per source and day, runs, success rate, offers per run, median steps, tokens, cost per run and the most common failure outcome (H3)             |
| M5.8  | Run inspection                   | `/admin/runs` filters by source, outcome, prompt version and day; a run's page shows each step's tool, arguments and result excerpt, with tokens and cost (H4)  |
| M5.9  | Agent sources live               | After the quota deploy, Amazon US, Amazon Japan and SMZDM are enabled in production and a search returns offers from at least two of them                       |
| M5.10 | An empty result is not a failure | An agent run that finds no listings does not count toward skipping the source                                                                                   |

### Tasks

- [ ] **5.1 Registry: empty is not failure.** `no_offers` returns zero offers without an error, so the registry does not skip the source.
- [ ] **5.2 Auth tables and gateway auth.** Goose migration for Better Auth's `auth` schema. Gateway: Better Auth with magic link (SMTP) and GitHub, account linking, admin role from `ADMIN_EMAILS`, identity headers to core, client IP from Caddy.
- [ ] **5.3 Quotas.** `app.search_quota`; the guard charges a search once, just before its first costly step; 429 with a clear message.
- [ ] **5.4 Product and admin API.** `GET /products/{id}` (offers and daily lowest price), `GET /admin/metrics`, `GET /admin/runs`, `GET /admin/runs/{id}`; admin role required. Contract and web types regenerated.
- [ ] **5.5 Web foundations.** Router, Zustand, services layer (API client, SSE, auth client), layout with sign-in state, currency setting.
- [ ] **5.6 Search and product pages.** Search module (query, stream, results, filters) and product page, by the structure in design.
- [ ] **5.7 Sign-in and admin pages.** Sign-in page (email link, GitHub); admin metrics, run list and run trace.
- [ ] **5.8 Deploy and go live.** New env values on the server (the owner sets the SMTP and GitHub secrets), check the three agent sources from the server with `source-smoke -browser-only`, enable them, verify M5.4 and M5.9 on the deployed site.
