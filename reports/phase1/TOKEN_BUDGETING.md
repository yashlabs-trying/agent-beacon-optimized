# Phase 1 — Token Budgeting (22 October 2026)

`go test -run '^TestContextBudgeting$|^TestSelectContextActuallyTruncatesBody$' -v ./internal/learning` passed on Mac and WSL2. The regression first failed: `SelectContext` counted a truncated body but returned the original full body, so actual selected text could exceed its declared budget. The repair returns a shortened copy, enforces `MaxTokens` even for the first item, and retains evidence on the selected memory. Input memories are not mutated.

On the existing 17 labeled queries, fixed top-5 context used 2,140 estimated tokens and included 85 items with 12/17 gold memories. With an 80-token/query cap, at most five items, and 40-token body truncation, the repaired selector used 1,358 estimated tokens across 60 items (1.58× fewer) and retained 12/17 gold. All 12 retained gold items had nonempty bodies, at least half of their original gold body by the test's estimate, and provenance evidence. This is the test's approximate title+body character/token estimate; serialization, metadata, and model-specific tokenizer costs are not counted.

The earlier 1,242-token, 11/17 result below was measured **before** the returned-body defect was fixed. It must not be treated as a valid delivered-prompt measurement. Larger relevance sets and actual model-token accounting remain open.

## Prior Phase 1 report (preserved below)

# Phase 1 — Token Budgeting

## Measured (`TestContextBudgeting`, 17 labeled queries)

| strategy | tokens | items | gold retained |
|---|---|---|---|
| fixed top-5 (untruncated) | 2,140 | 85 | 12/17 |
| budgeted (80 tokens, body trunc 40) | **1,242** | 52 | **11/17** |

Reduction **1.72×**; one gold memory lost (12→11). This is the known tradeoff: a
token saving that drops a relevant memory is **not** a quality improvement.

## Interpretation

- The budgeting here is a simple relevance-order + body-truncation policy. It
  preserves evidence for every included memory (only the body is truncated).
- Losing 1/17 gold suggests the default budget (80 tokens) is too tight for this
  set; a larger budget would recover it at higher token cost.
- Trust-aware and diversity-aware selection were **NOT TESTED**.

## Pass condition

Token selection is **CONDITIONAL PASS**: it reduces tokens, but the retention loss
must be quantified per budget and is not claimed as a quality win. Recommended
default: budgeted selection with a budget large enough to retain the top-K gold
(the 17-case set suggests ≥120 tokens), validated on a larger set in Phase 2.

## Reproduction

```bash
go test -run TestContextBudgeting -v ./internal/learning/
```
