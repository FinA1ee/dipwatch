# Agent harness log

Every prompt or harness change that is kept, with the evidence for it (spec H6). The design is in [design.md, "Agent harness"](design.md#agent-harness).

Until the eval set exists (Day 4), evidence is single live runs. From Day 4 on, each row links an eval report with precision, recall, steps and cost per store.

## Current version: browse-v5

| Area         | Setting                                                                                         |
| ------------ | ----------------------------------------------------------------------------------------------- |
| Model        | DeepSeek `deepseek-flash`, temperature 0, thinking off                                          |
| Budgets      | 15 steps, 30,000 tokens, 90 seconds per run                                                     |
| Wrap-up      | At 70% of steps or tokens, only `submit_offers` and `report_blocked` are offered                |
| Tools        | navigate, snapshot, click, type, press key, wait, find (max 15 results), plus submit and report |
| Page shaping | A snapshot after every page-changing tool; `compact` then `excerpt` to 8,000 characters         |
| History      | Only the latest page and the latest tool result stay in full; older results become a short note |

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
