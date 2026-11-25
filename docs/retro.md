# Week 1 retro

Dipwatch was built in seven days, from an empty repository to a deployed site: search across stores in China and Japan, an AI browsing agent with a measured harness, sign-in, tracking with Feishu alerts, and budget controls. This page records what worked, what did not, what it cost, and what comes next.

## What shipped

| Area          | Result                                                                                                                 |
| ------------- | ---------------------------------------------------------------------------------------------------------------------- |
| Search        | One store per search (owner can raise it), ranked by CNY value, results streamed as each store finishes                |
| Stores        | Kakaku.com (HTML parser, free), SMZDM (AI agent, about ¥0.06 per search), JD (affiliate API, waiting for an app key)   |
| Agent quality | browse-v7: precision 100%, recall 67% on 10 saved pages; injection page refused in every run                           |
| Matching      | 99.6% accuracy on 283 labelled listings (match-v2)                                                                     |
| Query gate    | 6 of 6 spelling groups map to one product; 7 of 7 non-products rejected                                                |
| Accounts      | GitHub sign-in live; email magic link built, waits for SMTP                                                            |
| Tracking      | Rules (below a price, drop %, back in stock, new store), notes, Feishu alerts, scheduled re-checks every 2 to 24 hours |
| Admin         | Per-source metrics, run list and trace, live agent log, budget limits form                                             |
| Operations    | Deploy on merge with rollback, daily backups (restore tested), spend breaker                                           |

## What worked

- **Measuring the agent before tuning it.** The eval set (Day 4) turned "the agent seems to miss listings" into numbers. The biggest jumps came from fixing what the model could see (joining split prices, a 12,000-character page view) and from fixing the labels, not from prompt wording.
- **Reading traces first.** On Day 3, three prompt versions tried to fix a problem that was in the harness: the model never received the page. The live agent log (Day 6) makes that kind of problem visible while it happens.
- **End-to-end checks in a real browser.** Every day's Playwright run found at least one bug the unit tests could not: a stream that reconnected 3,000 times, a missing CA bundle that broke every HTTPS call from the worker, a simulated price drop that picked a JPY offer as the cheapest.
- **Cost as a first-class requirement.** Cached answers are free, the matcher caches by title, a search costs at most one agent run, and the owner can change every limit without a deploy.

## What did not

- **Amazon and JD from a mainland server.** Both worked from a laptop and both block the Hangzhou server (Amazon US answers 503, Amazon Japan times out, JD shows a login wall). Store reachability should have been checked from the server on Day 1, not on Day 5.
- **The worker image is 1.9 GB.** Pushing it to Aliyun takes up to 20 minutes per deploy, most of the 10-minute budget of N2 on a bad day. Chromium dominates; a slimmer browser image or a separate, rarely changing base image would fix it.
- **Recall on Amazon Japan.** The agent submits 1 of 5 cards per page. It is the next eval target, though it matters less while Amazon is unreachable from the server.
- **Labels needed their own review.** The first "agent mistakes" on Day 4 were labelling mistakes. Every new eval case needs a human look.

## Cost so far

About ¥6 of LLM spend for the whole week. Most of it is evaluation runs (about ¥5 across eight agent-eval runs, three matcher evals and one intent eval); live searches and refreshes recorded in `agent_runs` add about ¥1 across the local and production databases. A cold SMZDM search costs about ¥0.07 including matching; a cached one costs nothing. The production daily budget starts at ¥5.

## Next

1. **JD live** once the JD Union app key is approved; Taobao through its affiliate API the same way.
2. **Email** sign-in and alerts once SMTP is set.
3. **A proxy outside mainland China** for Amazon, or drop Amazon for week 2.
4. **Recall on Amazon Japan** (browse-v9), measured on the eval set.
5. **A smaller worker image** so deploys stay well under 10 minutes.
6. **iOS app** with push notifications.
