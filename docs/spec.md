# Dipwatch spec

What Dipwatch does, for whom, and how we know it works. How it is built is in [design.md](design.md). Project-wide rules are in [constitution.md](constitution.md).

## Users

- **Visitor:** anyone with the URL. Can search and view results.
- **Member:** a visitor who signed in. Can also track products, write notes, set alert rules and choose notification channels.

## Scope for week 1

In scope: web app (installable PWA), Go core API and worker, TypeScript gateway with sign-in, at least 6 working sources, LLM matching, ranking, price history, tracking with email and Feishu alerts, CI/CD to the Aliyun ECS.

Out of scope for week 1, planned later: native iOS app with APNs push and Sign in with Apple, WeChat sign-in (needs a business entity), public domain and HTTPS (needs ICP filing), 淘宝 and 闲鱼 sources (need a logged-in local worker), MongoDB for raw listing archives.

## Functional requirements

Requirements use EARS form. IDs are referenced from [tasks.md](tasks.md).

### Search

- **S1.** When a visitor submits a query, the system shall return cached offers for that product within 1 second if a cache entry exists that is younger than 6 hours.
- **S2.** When no fresh cache entry exists, the system shall start a refresh for each enabled source and stream new offers to the client as each source finishes.
- **S3.** The system shall reject queries longer than 80 characters, queries containing URLs, and queries the intent check classifies as not a product, without contacting any source.
- **S4.** The system shall map different spellings of the same product ("5090", "RTX 5090", "nvidia geforce rtx 5090") to one canonical product, so they share one cache entry.
- **S5.** Each offer shall show: title in English, source name and region, price in the original currency, price converted to the selected currency, condition (new or used), availability, seller type where known, and a link to the listing.
- **S6.** The system shall exclude accessories, bundles of unrelated items and listings for a different product from results.
- **S7.** When a source fails or times out, the system shall return results from the other sources and shall not show the failure to the user.

### Ranking and currency

- **R1.** Offers shall be ranked by total price in CNY (price plus shipping where known), ascending, by default.
- **R2.** The user shall be able to filter by condition, region and source, and to sort by price or recency.
- **R3.** The user shall be able to pick a display currency. The default is CNY. Converted prices from other currencies shall be labeled approximate.
- **R4.** If the FX rate service is unreachable, the system shall use the most recent stored rates and keep working.

### Accounts

- **A1.** A visitor shall be able to sign in with email (magic link) or GitHub. Google sign-in is out of scope: the server cannot reach Google's token endpoint (checked on Day 2).
- **A2.** The same verified email used through different sign-in methods shall map to one member.
- **A3.** A member's tracked products, notes, alert rules, channels and currency shall be the same on every device they sign in from.

### Tracking and alerts

- **T1.** A member shall be able to track a product with one or more rules: price below an amount, drop by a percentage since tracking started, back in stock, or new offer from a source not seen before.
- **T2.** A member shall be able to attach a free-text note (up to 500 characters) to a tracked product.
- **T3.** While a product has at least one tracker, the system shall re-check its known offers every 6 hours by default, adapting between 2 and 24 hours based on how often the price changes, and shall search for new offers at least once a day. Products with no trackers shall not be refreshed on a schedule.
- **T4.** When a rule matches, the system shall notify the member through each enabled channel within 5 minutes of the refresh that detected it.
- **T5.** The system shall not send the same alert for the same offer and rule more than once per 24 hours.
- **T6.** Channels in week 1: email (default) and a Feishu bot webhook supplied by the member. The webhook host must be `open.feishu.cn` or `open.larksuite.com`.
- **T7.** A member shall be able to see the price history of a tracked product as a chart.

### Cost and abuse controls

- **C1.** Visitors shall be limited to 10 searches per day per IP that miss the cache. Members shall be limited to 30 per day. Cache hits are not limited.
- **C2.** A member shall be limited to 20 tracked products.
- **C3.** Each browsing-agent run shall stop after 15 tool steps, 30,000 tokens or 90 seconds, whichever comes first.
- **C4.** When daily LLM spend reaches 80% of the daily budget, the browsing agent shall run only for products that already have trackers. At 100%, the system shall serve cached data only until the next day.
- **C5.** Every LLM call shall be recorded with model, tokens, estimated cost, duration and outcome.

## Non-functional requirements

- **N1.** All services shall run together on a 2 vCPU, 4 GB server, with at most 2 concurrent browser sessions.
- **N2.** A merge to `main` shall be live on the server within 10 minutes, with no manual steps.
- **N3.** A failed deploy health check shall roll back to the previous image automatically.
- **N4.** A new contributor shall be able to run the full stack locally from the README with one command after installing Go, Node, pnpm and Docker.
- **N5.** No secret shall be committed. CI shall run a secret scanner on every pull request.

## Success metrics for week 1

| Metric                          | Target                                                       |
| ------------------------------- | ------------------------------------------------------------ |
| Working sources                 | At least 6, with at least 2 in China and 3 overseas          |
| Filter accuracy                 | At least 90% on a hand-labeled set of 30 "RTX 5090" listings |
| Cached search latency           | Under 1 second at p95                                        |
| Cold search, first offers shown | Under 10 seconds                                             |
| Alert delay                     | Under 5 minutes from refresh to email                        |
| Deploy time                     | Merge to live in under 10 minutes                            |
| LLM cost per cold search        | Measured and reported, target under ¥0.10                    |
