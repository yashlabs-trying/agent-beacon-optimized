# Phase 5 — Self-Improvement (Suite A)

## Suite A, per category

| Category | A | B | C | D | E |
|---|---:|---:|---:|---:|---:|
| A1 learning from failure | 0/20 | 2/20 | 5/20 | 5/20 | 0/20 |
| A2 supersession | 0/20 | 19/20 | 19/20 | 19/20 | 19/20 |
| A3 knowledge transfer | 0/20 | 20/20 | 2/20 | 2/20 | 20/20 |
| A4 recovery (post-restart state) | 0/20 | 14/20 | 14/20 | 14/20 | 20/20 |
| A5 long session | 0/20 | 20/20 | 20/20 | 20/20 | 20/20 |
| **Suite A total** | 0/100 | 75/100 | 60/100 | 60/100 | 79/100 |

## Findings

1. **A2 supersession: all memory arms 19/20** — memory helps (vs A 0/20) and the four
   memory configurations are equivalent. The one miss is a model slip. This is a strong,
   consistent win for memory over no-memory, but **not a V6-vs-original differentiator**.
2. **A3 knowledge transfer is a severe semantic regression**: original 20/20 vs
   semantic/intelligent **2/20**. The gold fact ("all Go services use UTC timestamps") is
   retrieved by lexical (which returns only the exact-matching note) but semantic also
   returns an unrelated "uses tabs" note, and the model is derailed. **This single category
   costs the semantic/intelligent arms 18 tasks and is the dominant reason D does not beat B.**
3. **A1 learning from failure is invalid as graded**: the oracle scores 0/20 (the expected
   token is never produced even with the correct fix note), so A1 does not test memory.
   It is flagged invalid (not replaced) — see `FAILURE_ANALYSIS.md`.
4. **A4/A5 are memory-positive but non-discriminating** (all memory arms equal).

## Answer

- Does memory help agents avoid repeating mistakes / use feedback? **PARTIALLY (A2 yes;
  A1 invalid, so not established)**.
- Does V6 learn better than original? **NO** — equal on A2/A4/A5, and **worse on A3**.
- The A3 failure is the single most important actionable finding: semantic retrieval's
  extra context must be filtered by relevance to the *question*, not just to the query.
