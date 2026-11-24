# Day 1 spike: can we fetch each source?

One plain HTTP GET per store search page for "RTX 5090", sent once from the dev machine on 2026-11-23, with a desktop browser user agent and 2 seconds between requests. No retries and no attempt to get past blocks. Program: [`tools/spike/sources`](../../tools/spike/sources/main.go), run with `make spike`.

This only tells us what plain HTTP gets. The browsing agent (real Chromium, Day 3) and the server's network (Aliyun, mainland China, Day 2) can behave differently, so each row is a starting point, not a verdict.

## Results

| Source              | Region | HTTP | Size   | Price matches | "5090" mentions | JSON-LD | Block signal |
| ------------------- | ------ | ---- | ------ | ------------- | --------------- | ------- | ------------ |
| JD.com 京东         | CN     | 200  | 2 KB   | 0             | 0               | no      | `验证`       |
| Taobao 淘宝         | CN     | 200  | 33 KB  | 0             | 0               | no      | -            |
| Xianyu 闲鱼         | CN     | 200  | 11 KB  | 0             | 0               | no      | -            |
| Pinduoduo 拼多多    | CN     | 200  | 47 KB  | 0             | 0               | no      | -            |
| SMZDM 什么值得买    | CN     | 202  | 0 KB   | 0             | 0               | no      | -            |
| eBay                | US     | 403  | 1 KB   | 0             | 0               | no      | -            |
| Newegg              | US     | 403  | 5 KB   | 0             | 3               | no      | -            |
| Amazon US           | US     | 200  | 637 KB | 1             | 205             | no      | -            |
| Best Buy            | US     | 200  | 6 KB   | 0             | 0               | no      | -            |
| B&H Photo           | US     | 403  | 5 KB   | 0             | 3               | no      | -            |
| PCPartPicker        | US     | 403  | 5 KB   | 0             | 3               | no      | -            |
| Amazon Japan        | JP     | 200  | 940 KB | 0             | 362             | no      | -            |
| Kakaku.com 価格.com | JP     | 200  | 243 KB | 0             | 317             | yes     | -            |

"Price matches" uses a simple currency regex. It misses prices split across HTML elements, which is why Amazon shows almost none despite returning full result pages.

## What the responses mean

- **Full result pages:** Amazon US, Amazon Japan and Kakaku return server-rendered results. Kakaku also embeds JSON-LD structured data, which is the easiest kind of page to parse.
- **Verification page:** JD answers with a 2 KB page containing 验证 (verification). Plain HTTP is blocked.
- **JavaScript shells:** Taobao, Xianyu and Pinduoduo return a page with no results in it. The results load by script, after login for Taobao and Xianyu.
- **Bot challenge:** Newegg, B&H and PCPartPicker return the same 5 KB 403 page, typical of a CDN bot check. SMZDM returns an empty 202, also typical of a script challenge. A real browser may pass these.
- **Blocked or redirected:** eBay returns 403 to plain HTTP, but has an official API. Best Buy returns a 6 KB page without results, likely a region redirect for non-US visitors.

## Starting strategy per source

The spike adds one tier to the design: **html**, a deterministic parser for pages that return results without script (CSS selectors or JSON-LD). It costs no tokens, so it is tried before the agent. It is the same mechanism the agent's saved recipes use for re-checks.

| Source       | Starting strategies (in order)         | Week 1 status                                                    |
| ------------ | -------------------------------------- | ---------------------------------------------------------------- |
| eBay         | api (Browse API)                       | Day 2, needs developer keys                                      |
| Kakaku.com   | html (JSON-LD)                         | Day 3                                                            |
| Amazon Japan | html, agent                            | Day 3                                                            |
| Amazon US    | html, agent                            | Day 3                                                            |
| JD.com       | agent                                  | Day 3. If the agent also hits verification, defer with Taobao    |
| SMZDM        | agent                                  | Day 3                                                            |
| Newegg       | agent                                  | Day 3                                                            |
| B&H Photo    | agent                                  | Day 3                                                            |
| PCPartPicker | agent                                  | Day 3. Aggregates US retailers, so one source covers many stores |
| Best Buy     | api (Products API)                     | Only if keys are granted and the API accepts non-US callers      |
| Taobao       | api (淘宝联盟 affiliate API)           | After week 1. Needs a 淘宝客 affiliate account and app approval  |
| Xianyu       | agent with a logged-in browser session | After week 1. See design, "Sources that need a login"            |
| Pinduoduo    | agent with a logged-in local worker    | Deferred                                                         |

That gives 6 to 8 candidates for the week 1 target of at least 6 working sources, with at least 2 in China (JD, SMZDM) and 3 overseas.

## Day 2 follow-up

- **Kakaku** redirects to `search.kakaku.com`. Its JSON-LD turned out to be only a breadcrumb, so the adapter reads the HTML instead (`p-item` blocks with `p-item_name` and `p-item_priceNum`). A live search for "RTX 5090" returned 44 offers, and it is the first enabled source.
- **eBay** is deferred. It needs a developer account and keyset approval, and its listings are mostly auctions and resellers. The `ebay-us` row stays in the database, disabled.
- Kakaku results include laptops and PCs with an RTX 5090, not only the card. Filtering those out is the LLM matcher's job (Day 4, spec S6).

## Day 2: the same check from the server

Run once from the ECS (Aliyun, Hangzhou, 2026-11-24). Store pages:

| Source                          | From the dev machine | From the server        |
| ------------------------------- | -------------------- | ---------------------- |
| Kakaku.com                      | 200, results         | 200, results (244 KB)  |
| Amazon US                       | 200, results         | 200, results (636 KB)  |
| Amazon Japan                    | 200, results         | **Timeout after 20 s** |
| Best Buy                        | 200, no results      | **Connection error**   |
| JD.com                          | Verification page    | Verification page      |
| Taobao, Xianyu, Pinduoduo       | Script shell         | Script shell           |
| SMZDM                           | Empty 202            | Empty 202              |
| eBay, Newegg, B&H, PCPartPicker | 403                  | 403                    |

Other services the server needs:

| Service                                     | Result                                                                 | Consequence                                                   |
| ------------------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------- |
| DeepSeek, Qwen (DashScope), Kimi (Moonshot) | Reachable, under 1 s                                                   | Any of the three can be the server LLM                        |
| OpenAI                                      | Unreachable                                                            | Local experiments only, as planned                            |
| Google OAuth (`oauth2.googleapis.com`)      | Unreachable                                                            | **No Google sign-in.** Email and GitHub only (spec A1)        |
| GitHub, GitHub API                          | Reachable                                                              | GitHub sign-in works                                          |
| Aliyun ACR (Hangzhou)                       | Reachable, 40 ms                                                       | Images come from ACR, as planned                              |
| Docker Hub                                  | Unreachable                                                            | Confirms the ACR copy of Postgres is needed                   |
| Go module proxy                             | Unreachable                                                            | Never build Go on the server, as planned                      |
| FX API (frankfurter)                        | Reachable; `api.frankfurter.app` redirects to `api.frankfurter.dev/v1` | Use `https://api.frankfurter.dev/v1/latest?base=CNY` directly |

Amazon Japan works from the dev machine but not from the server, so it may need the agent or a local worker. Kakaku and Amazon US work from both.

## Open risks

- **China coverage is thin.** Only JD and SMZDM are candidates without login, and JD showed verification. If both fail under the agent, week 1 has no China source, and the logged-in local worker for Taobao moves up.
- **Server network.** Overseas sites often block Chinese datacenter IPs more aggressively than a home connection. Day 2 repeats this spike from the ECS.
- **Terms of use.** Automated access is low volume and personal, but some sites forbid it in their terms. Sources stay individually switchable in the `sources` table.
