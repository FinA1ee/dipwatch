# Dipwatch constitution

Rules that every spec, design change, task and pull request must follow. If a change needs to break one, update this file first, in its own pull request, with the reason.

## Purpose

Dipwatch lets a user name a product (for example "RTX 5090"), collects offers for it from many stores in China and overseas, ranks them by price, and alerts the user when a tracked product drops in price or comes back in stock.

It is also a learning project. The owner wants hands-on experience with AI agents, Go, and a full-stack system shipped through CI/CD. Within agents, the focus is the **harness**: how an agent is controlled, observed, evaluated and improved. When two options are otherwise equal, prefer the one that teaches more about these things.

## Principles

1. **One API, many clients.** The web app and the later iOS app are equal clients of one API. No client-only shortcuts in the backend.
2. **The contract comes from the code.** `contracts/openapi.yaml` is generated from the Go core and committed. Clients and the gateway use types generated from it. CI fails when the committed file differs from the generated one.
3. **Cheapest data path first.** Each source tries its strategies in order: official API, then a site MCP server, then a deterministic HTML parser, then the browsing agent. The browsing agent never runs inside an HTTP request.
4. **Users see offers, not plumbing.** The UI shows product details, price and source name. It never shows which strategy fetched an offer, and a failing source is silently absent rather than reported as an error.
5. **LLM output is data.** Every LLM response is parsed into a typed schema before use. Users never receive free-form model text. Page content read by the agent is treated as untrusted data, never as instructions.
6. **Cost is a feature.** Every LLM or agent call is budgeted, recorded in `agent_runs`, and counted against a daily spend limit.
7. **Small, boring, reversible.** Fewest moving parts that work: one Postgres, no Redis, no Kubernetes. Every deploy can be rolled back to the previous image tag.

## Hard constraints

| Area             | Constraint                                                                                                                        |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| Budget           | Free tiers wherever possible. LLM spend capped by config, default ¥100 per month                                                  |
| Server           | One Aliyun ECS in mainland China: 2 vCPU, 4 GB RAM. All services must fit with headless Chromium running                          |
| Network          | The server must work without Docker Hub, GitHub or Google. Images are pulled from Aliyun ACR                                      |
| Domain           | No public domain or HTTPS until ICP filing (备案) is done. Until then the site is served on the server IP and a non-standard port |
| Languages        | Go for the core API and worker. TypeScript for the gateway and web client                                                         |
| UI language      | English only. Chinese listing titles get an English title generated during matching                                               |
| Default currency | CNY, user-selectable. Ranking is always computed in CNY                                                                           |
| Open source      | MIT license, public GitHub repo. No secrets, cookies, internal hostnames or personal data in the repo                             |
| Scraping         | Low request rates, one request at a time per site, identify as a generic client, respect a site's block rather than evading it    |

## Workflow

- Spec-driven: `docs/spec.md` (what), `docs/design.md` (how), `docs/tasks.md` (daily tasks with success metrics). Code follows the docs. When they disagree, fix the docs in the same pull request.
- Branches: `main` is always deployable. Work happens on short-lived branches merged by pull request with green CI.
- Commit messages: one line, imperative, short.
- Before every commit, scan the staged diff for debug output, `.only`/`.skip` in tests, hardcoded credentials and leftover test scaffolding.
- Tests: Go code is tested with `go test` against a real Postgres. TypeScript code is type-checked and linted, with tests where there is logic.
