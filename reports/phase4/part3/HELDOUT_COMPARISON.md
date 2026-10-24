# Phase 4C — Held-out Comparison (Stage F)

Frozen held-out corpus: `phase4c_heldout_v1.json`, 18 cases, SHA-256 `6461AF90…`,
disjoint from the development corpus (different projects, entities, values, phrasings).
Hashed before any held-out run. Arms: none, original, v6_semantic, v6_intelligent, oracle.

## 1. Results

| Arm | Correct | Accuracy | 95% CI |
|---|---:|---:|---|
| none | 1/18 | 0.056 | — |
| original Beacon | 2/18 | 0.111 | [0.031, 0.328] |
| v6_semantic | 10/18 | 0.556 | [0.337, 0.754] |
| **v6_intelligent** | **14/18** | **0.778** | [0.548, 0.910] |
| oracle | 15/18 | 0.833 | [0.608, 0.942] |

## 2. Paired comparisons (exact McNemar)

| Comparison | A / B / ties | p |
|---|---|---:|
| v6_intelligent vs v6_semantic | **4 / 0 / 14** | 0.125 |
| v6_intelligent vs original | 13 / 1 / 4 | **0.0018** |
| v6_intelligent vs none | 13 / 1 / 4 | **0.0018** |
| v6_intelligent vs oracle | 0 / 1 / 17 | 1.000 |
| v6_semantic vs original | 9 / 1 / 8 | **0.0215** |

## 3. Category movement (held-out)

| Category | none | semantic | intelligent | oracle |
|---|---:|---:|---:|---:|
| contradiction (5) | 1 | 2 | **3** | 4 |
| ambiguity (4) | 1 | 4 | 4 | 4 |
| adversarial (4) | 0 | 1 | **4** | 4 |
| decision (3) | 0 | 3 | 3 | 3 |
| control (2) | 0 | 0 | 0 | 0 (invalid) |

## 4. Reading

- **Direction is consistent with the development set:** the intelligent layer beats raw
  semantic retrieval **4–0 with no regressions** (dev was 6–0), and reaches **oracle
  parity** (0–1–17, p=1.0).
- **Not statistically significant at n=18** (p=0.125). A single seed, 18 tasks. This is a
  directional held-out result, **not** proof of superiority.
- **Injection fix generalises:** adversarial 1/4 → 4/4 held-out (0 attacks succeed with
  the layer).
- **Contradiction** improves but does not reach oracle (3 vs 4): the remaining miss is the
  model reasoning failure (`h_...` version/scoping), not retrieval.

## 5. What would make this conclusive

≥100 held-out tasks, ≥3 seeds, paired analysis treating seeds as dependent within task,
and pre-registered thresholds — the plan in `LIMITATIONS_AND_NEXT_STEPS.md`.

## Verdict

Held-out comparison: **CONDITIONAL PASS** — the intelligent layer is directionally better
than semantic retrieval (4–0) and at oracle parity, but the effect is not significant at
n=18. No regression was observed.
