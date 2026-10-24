# Phase 4B — Statistical Analysis (Development Pilot)

- Runs: 20 cases × 7 arms = 140 model executions (qwen2.5-coder:7b).
- Independent unit: the logical task (case). Seeds: 1 (calibration; multi-seed deferred).
- Raw: `RAW_EVIDENCE/agent_pilot.json`, `RAW_EVIDENCE/analysis.{json,txt}`.

## 1. Per-arm accuracy (Wilson 95% CI)

| Arm | Correct | Accuracy | 95% CI |
|---|---:|---:|---|
| none | 2/20 | 0.100 | [0.028, 0.301] |
| original (lexical) | 2/20 | 0.100 | [0.028, 0.301] |
| v6_lexical | 2/20 | 0.100 | [0.028, 0.301] |
| v6_hashed | 15/20 | 0.750 | [0.531, 0.888] |
| v6_hybrid | 15/20 | 0.750 | [0.531, 0.888] |
| v6_neural | 16/20 | **0.800** | [0.584, 0.919] |
| oracle | 19/20 | 0.950 | [0.764, 0.991] |

## 2. Paired comparisons (A-wins / B-wins / ties, exact McNemar)

| A vs B | A wins | B wins | ties | p |
|---|---:|---:|---:|---:|
| v6_neural vs original | 14 | 0 | 6 | **0.0001** |
| v6_neural vs none | 14 | 0 | 6 | **0.0001** |
| v6_hashed vs original | 13 | 0 | 7 | **0.0002** |
| v6_neural vs v6_hashed | 1 | 0 | 19 | 1.0000 |
| oracle vs v6_neural | 4 | 1 | 15 | 0.3750 |
| v6_lexical vs original | 0 | 0 | 20 | 1.0000 |

## 3. Required-evidence Recall@K (mean)

| Arm | Recall |
|---|---:|
| none / original / v6_lexical | 0.100 |
| v6_hashed / v6_hybrid / v6_neural | **1.000** |
| oracle | 1.000 |

## 4. Latency / tokens (mean per case)

| Arm | agent s | prompt tok | gen tok |
|---|---:|---:|---:|
| none | 1.63 | 73 | 8 |
| original | 0.54 | 73 | 8 |
| v6_lexical | 0.38 | 73 | 8 |
| v6_hashed | 1.57 | 117 | 7 |
| v6_hybrid | 1.11 | 117 | 7 |
| v6_neural | 1.35 | 117 | 7 |
| oracle | 1.28 | 88 | 7 |

## 5. Reading

- **Semantic retrieval clearly beats the shipped lexical default** on natural-language
  memory tasks: +65–70 points (p ≤ 0.0002). This is the largest effect in Phase 4A/4B.
- **Neural ≈ hashed** on this dev set (1–0–19, p=1.0): the extra cost of the neural
  sidecar buys no accuracy here. Neural's advantage must be shown on harder held-out
  cases or dropped.
- **V6_neural is statistically indistinguishable from oracle** (p=0.375): retrieval is
  no longer the bottleneck on these cases — the residual gap is reasoning.
- **original == v6_lexical** (0–0–20): confirms the audit; the base lexical engine is
  unchanged between systems.
- Confidence intervals are wide (n=20); these are calibration numbers, **not** a
  held-out victory claim.

## 6. Caveats

- Single seed; 20 cases; development split only. No held-out result exists yet.
- The abstain category is non-discriminating (all arms 2/2) because a memory-less agent
  also abstains — a known benchmark-design weakness flagged for the held-out split.
