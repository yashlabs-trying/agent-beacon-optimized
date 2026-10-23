# Phase 5 — TOP_25_REGRESSIONS

The 25 most important verified regressions/failure cases. There are exactly **22
regressions** (original correct, V6 intelligent wrong); the remaining entries are the
highest-impact shared failures. No product fixes were implemented (frozen evaluation).

Legend: **Reg** = original correct, V6 wrong. **Shared** = all memory arms (incl. oracle)
fail. Root causes are from trace inspection.

## A. Knowledge-transfer over-retrieval regressions (18 cases: a3-41…a3-60, minus 47,57)

| # | Task | Repro | Expected | Actual (V6 C/D) | Root cause | Original better? | Fix | Impact | Pri |
|---|---|---|---|---|---|---|---|---|---|
| 1–18 | `a3-4x/5x/6x` | query "Go services"; gold note "All Go services use UTC timestamps"; distractor "The X team uses tabs, not spaces." | "utc" | wrong token / confused | semantic retrieval returns the distractor; not filtered | **YES (B 20/20 vs D 2/20)** | filter context by question relevance | −18 tasks | P0 |

Representative row (the other 17 are the same mechanism with different entities):

| # | Task | Repro | Expected | Actual | Root cause | Orig better | Fix | Impact | Pri |
|---|---|---|---|---|---|---|---|---|---|
| 1 | `a3-41` | as above | `utc` | wrong | over-retrieval | YES | question-relevance filter | 1 task | P0 |

## B. Ambiguity over-answer regressions (4 cases: b2-124, b2-126, b2-130, b2-136)

| # | Task | Repro | Expected | Actual | Root cause | Orig better | Fix | Impact | Pri |
|---|---|---|---|---|---|---|---|---|---|
| 19–22 | `b2-12x/13x` | unresolvable deploy task; hint "team sometimes uses canary" | abstain | guesses an env | semantic surfaces a hint; model over-answers | **YES (B 20/20 vs D 16/20)** | stronger sufficiency signal | −4 tasks | P1 |

## C. Highest-impact shared failures (not regressions, but unsolved by all arms)

| # | Task(s) | Category | Expected | Actual | Root cause | Orig better? | Fix | Impact | Pri |
|---|---|---|---|---|---|---|---|---|---|
| 23 | `b1-*` (12/20) | contradiction | correct value / abstain | wrong or guessed | **model reasoning** (oracle 8/20) | No (all equal) | reasoning-support prompt with provenance | 12 tasks | P0 |
| 24 | `b3-*` (12/20) | multi-hop | combined value | wrong | **model reasoning** (oracle 7/20) | No (B worse 0/20) | reasoning-support prompt | 12 tasks | P0 |
| 25 | `c1-*` (7/20) | coding | tests pass | failing code | **model coding error** (identical across arms) | No (all equal) | stronger model / tool loop | 7 tasks | P2 |

## Summary

| Class | Count | Dominant cause |
|---|---:|---|
| Regressions (B > D) | 22 | over-retrieval (18) + ambiguity over-answer (4) |
| Shared model-reasoning failures | 24 (b1 12 + b3 12) | model ceiling, not memory |
| Shared model coding failures | 7 | model ceiling |
| **Invalid task (flagged)** | 20 (A1) | oracle 0/20 → not memory-testable; excluded from headline |

## Notes

- No security regression is listed: leaks were 0/40 for every arm. The B5 semantic-only
  regression (C 5/20 vs B 14/20) is repaired by the intelligent layer (D 20/20), so it is
  not a D-vs-B regression.
- The single most valuable fix is **question-relevance filtering** (18+4 tasks), followed
  by **reasoning support** for contradictions/multi-hop (24 tasks).
