# Phase 4C — Cost and Context Analysis (Benchmark G)

## 1. Context tokens by arm (dev, mean per case)

| Arm | prompt tokens | agent seconds | notes |
|---|---:|---:|---|
| none | 75 | 0.53 | no evidence |
| original | 75 | 0.34 | lexical returns nothing |
| v6_semantic | 98 | 0.43 | all retrieved memories |
| v6_intelligent | **93** | **0.30** | fewer, curated memories |
| oracle | 85 | 0.26 | minimal gold evidence |
| ab_trust | 94 | 0.26 | |
| ab_injection | 95 | 0.25 | |

**The intelligent layer uses fewer tokens than raw semantic retrieval (93 vs 98) while
achieving higher accuracy (26/35 vs 20/35).** It is not token-reduction for its own sake:
it drops stale/untrusted memories and therefore admits less irrelevant context.

## 2. Equal-budget comparison

All memory arms receive the same retrieval limit (K=8) and identical prompt/scoring. The
selection layer only changes *which* of the retrieved memories are shown. No arm gets a
larger context allowance, so the comparison is fair (documented; no unequal budgets).

## 3. Multiple token budgets

The 256/512/1024/2048-token budget matrix is **BLOCKED** in this phase. The existing
token-budget selector (`SelectContext`, `search_context.go`) is separate from the trust
layer and remains opt-in; Phase 3A/3B established it loses 3/109 findings. Combining the
trust layer with a hard token budget is a next-phase experiment.

## 4. Cost per successful task

Using mean agent seconds / accuracy as a crude proxy (not true monetary cost):

| Arm | accuracy | mean agent s | s per correct task |
|---|---:|---:|---:|
| v6_semantic | 0.571 | 0.43 | 0.75 |
| v6_intelligent | 0.743 | 0.30 | **0.40** |
| oracle | 0.771 | 0.26 | 0.34 |

The intelligent layer is **cheaper per correct task** than raw semantic retrieval because
it is both more accurate and faster (less context). Retrieval-side latency is unchanged
(the layer is CPU-only, sub-millisecond at K=8).

## 5. Important findings lost / contradictions preserved

- No required-evidence loss: mean required-evidence recall stays **1.000** for
  semantic and intelligent (trust-only drops to 0.971 by design).
- Unresolved conflicts are preserved (both sides kept) and signalled, not silently
  resolved.

## Verdict

Cost/context efficiency: **PASS** — higher accuracy at equal-or-lower tokens.
Hard token-budget matrix: **BLOCKED**.
