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
