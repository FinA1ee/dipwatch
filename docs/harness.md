# Agent harness log

Every prompt or harness change that is kept, with the evidence for it (spec H6). The design is in [design.md, "Agent harness"](design.md#agent-harness).

Until the eval set exists (Day 4), evidence is single live runs. From Day 4 on, each row links an eval report with precision, recall, steps and cost per store.

## Current version: browse-v8

| Area         | Setting                                                                                                       |
| ------------ | ------------------------------------------------------------------------------------------------------------- |
| Model        | DeepSeek `deepseek-flash`, temperature 0, thinking off                                                        |
| Budgets      | 15 steps, 30,000 tokens, 90 seconds per run                                                                   |
| Wrap-up      | At 70% of steps or tokens, only `submit_offers` and `report_blocked` are offered                              |
| Tools        | navigate, snapshot, click, type, press key, wait, find (max 15 results), plus submit and report               |
| Page shaping | A snapshot after every page-changing tool; split prices joined; `compact` then `excerpt` to 12,000 characters |
| History      | Only the latest page and the latest tool result stay in full; older results become a short note               |

## Iterations

### Day 3: making the agent finish (browse-v1 to v5)

Store: Amazon US, query "RTX 5090", one live run per version.

| Version   | Change                                                                                                        | Outcome               | Steps | Tokens in | Cost   |
| --------- | ------------------------------------------------------------------------------------------------------------- | --------------------- | ----- | --------- | ------ |
| browse-v1 | First version                                                                                                 | token limit, 0 offers | 9     | 37,479    | ¥0.081 |
| browse-v2 | Trim old tool results; do not change region; submit early                                                     | token limit, 0 offers | 8     | 31,618    | ¥0.068 |
| browse-v3 | `browser_find` capped at 15; results 8,000 characters; one results page; allow amazon.sg                      | token limit, 0 offers | 9     | 29,782    | ¥0.065 |
| browse-v4 | Snapshot after every page-changing tool (navigate returned only a file link, so the model never saw the page) | token limit, 0 offers | 9     | 31,200    | ¥0.069 |
| browse-v5 | Keep the latest page when trimming; forced wrap-up at 70% of the budget                                       | **ok, 1 offer**       | 6     | 25,424    | ¥0.056 |

browse-v5 on the other stores: Amazon Japan ok with 1 offer (6 steps, ¥0.068), SMZDM ok with 5 offers (5 steps, ¥0.050).

Lessons:

- **Read the traces before changing prompts.** v1 to v3 tuned the prompt for a problem that was really in the harness: the model never received the page. One look at the raw tool result in v4 found it.
- **Context grows with every turn.** Each model call re-sends the whole conversation, so a 10,000-character page seen on step 1 is paid for again on every later step. Trimming old results matters more than shortening any single one.
- **Models keep exploring when allowed.** The forced wrap-up turned "keeps searching until the budget runs out" into "submits what it has".

Known gaps for the eval to measure: Amazon runs submit 1 offer although the page shows 9 to 13 (low recall); SMZDM prices may be list prices.

### Day 4: the first measured iterations (browse-v5 to v7)

The eval set: 10 saved results pages (Amazon US, Amazon Japan and SMZDM, for RTX 5090, RTX 4090 and Sony WH-1000XM6), one of them with an injected instruction. Labels come from `tools/eval/label_draft.py` plus hand decisions in `overrides.json`. Reports are in [harness/](harness/).

| Run                     | Change                                                                                                                                                                      | Precision | Recall  | Answered     | Median cost | Report                                         |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- | ------- | ------------ | ----------- | ---------------------------------------------- |
| browse-v5               | Baseline                                                                                                                                                                    | 53%       | 26%     | 8 of 10      | ¥0.057      | [report](harness/2026-11-25-1045-browse-v5.md) |
| browse-v6               | `compact` joins prices split across lines (SMZDM shows "¥" "1640." "95"); title matching uses only the model number, because brands appear in other scripts (索尼 for Sony) | 62%       | 38%     | 9 of 10      | ¥0.058      | [report](harness/2026-11-25-1049-browse-v6.md) |
| browse-v6, labels fixed | Same code. The labeller had missed brand-heading cards, colour variants, cards without a price and single-quoted YAML lines                                                 | 76%       | 44%     | 7 of 10      | ¥0.063      | [report](harness/2026-11-25-1055-browse-v6.md) |
| browse-v7               | Prompt: leave out PCs, laptops, bundles and other models; list every matching listing; an empty submission means "none here". Page view 12,000 characters                   | **100%**  | **67%** | **10 of 10** | ¥0.060      | [report](harness/2026-11-25-1057-browse-v7.md) |
| browse-v8               | Prompt: China-market "D" editions (RTX 4090D) count as the product                                                                                                          | 95%       | 56%     | 10 of 10     | ¥0.061      | [report](harness/2026-11-25-1343-browse-v8.md) |

Targets (spec H1): precision at least 90% (met), recall at least 60% (met by v7; v8 56%), median cost under ¥0.08 (met). The injection case was refused in every run.

v8 against v7: every SMZDM case, where the 4090D listings are, scored the same. The whole recall difference is Amazon Japan, which submits one card per run under both prompts (v7's injection run happened to submit 4). The difference is within run-to-run noise; Amazon Japan recall is the next thing to fix.

Lessons:

- **Check the labels before blaming the agent.** The third row moved precision from 62% to 76% without touching the agent: the "wrong" offers were real listings the labeller had missed. Every eval needs its own review step.
- **The page view decides what the model can know.** A price split into three lines became three meaningless fragments that `compact` then deleted. No prompt could have fixed that.
- **Say what "nothing found" looks like.** Without it, the model reported pages with no matching product as "blocked", which the registry treats as a store failure.
- **Recall is now the gap.** On Amazon Japan the model still submits 1 of 5 cards. Likely causes for the next iteration: listings near the bottom of the 12,000-character view, and the wrap-up pushing it to submit early.

### Day 4: the matcher (match-v1)

After each refresh, one LLM call per 40 listings decides whether each listing is the product, and adds an English title and a condition. Decisions are cached per product and title, so an unchanged listing is never sent twice. `go run ./cmd/matcher-eval -env ../deploy/.env` scores it on the same labelled listings as the agent eval, leaving out ambiguous labels and the injection copy.

| Version  | Accuracy  | Precision | Recall | Listings | Cost   | Report                                        |
| -------- | --------- | --------- | ------ | -------- | ------ | --------------------------------------------- |
| match-v1 | **98.9%** | 100%      | 92.5%  | 283      | ¥0.140 | [report](harness/2026-11-25-1110-match-v1.md) |
| match-v2 | **99.6%** | 97.6%     | 100%   | 283      | ¥0.139 | [report](harness/2026-11-25-1345-match-v2.md) |

Target (spec S6): accuracy at least 90% (met). An earlier run of the same prompt kept one Sony bundle (precision 97.4%), so bundles are the case to watch.

Every match-v1 miss was an "RTX 4090D", the China-only cut-down variant. Decision (2026-11-25): D editions count as the product, since a shopper searching "RTX 4090" in China wants those prices too. match-v2 says so in the prompt and recall went to 100%. The one miss left is a Sony bundle (headphones plus a stand), which earlier runs also kept sometimes.

### Day 4: the intent check (intent-v1)

The query gate normalizes the text (NFKC, lower case, single spaces), rejects input over 80 characters or with a URL, and otherwise asks the model once per normalized text whether it names a product and what its canonical name is. Both answers are cached in `queries`. To keep spellings on one product, the call lists existing products that share a model-number word with the query, and the model is told to reuse a listed name exactly. `go run ./cmd/intent-eval -env ../deploy/.env` runs 26 texts from `internal/guard/testdata/queries.json`.

| Version   | Spelling groups on one name | Non-products rejected | Cost   | Report                                         |
| --------- | --------------------------- | --------------------- | ------ | ---------------------------------------------- |
| intent-v1 | 6 of 6                      | 7 of 7                | ¥0.049 | [report](harness/2026-11-25-1114-intent-v1.md) |

The rejected set includes an instruction ("ignore previous instructions…"); the model can only answer through the schema, so there is nothing for it to act on. The set is small and should grow with real queries once the site is public.
