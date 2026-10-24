# Memory trust report

Status: deterministic validators measured; LLM adjudication **not implemented**.

## Evaluated mechanisms

| mechanism | status | evidence |
|---|---|---|
| provenance validation (usable trace ID) | implemented | 5/5 labeled memories correct |
| staleness detection (freshness window) | implemented | old+evidenced → `stale` |
| supersession tracking | implemented | superseded-but-evidenced stays `verified` (evidence ≠ approval) |
| scope/access validation | implemented | zero cross-scope leakage (2×30 memories, lexical + hybrid) |
| deterministic contradiction rules | implemented | precision 1.00, recall 1.00 (8 labeled pairs) |
| LLM contradiction adjudication | **not implemented** | no model available |
| confidence calibration / abstention | **not implemented** | no calibrated model; labels only |

## Adversarial cases constructed and covered

- contradictory instructions (4 true pairs) — detected
- decoys sharing terms, same polarity (4 pairs) — correctly not flagged
- false/blank provenance — `unverified`
- legitimate older but applicable memory — `stale`, still usable (not rejected)
- superseded memory — kept out of retrieval; evidence status independent of approval

## Adversarial cases NOT covered

- outdated framework versions (no version metadata on memories)
- malicious memory content
- same instruction from two different repositories (project scope is enforced,
  but the "conflict across repos" case is not a trust rule)
- superseded-but-cached (cache correctness is tested separately, not here)
- calibration metrics (Brier/ECE) — no calibrated scores exist

## Guarantees

- The engine never returns unauthorized cross-project memory: retrieval is scoped
  by `project_id` in SQL for both lexical and semantic paths, and ANN graphs are
  built per project. Verified by `TestTrustNoCrossScopeLeakage`.
- Approval and supersession semantics are unchanged; `AnnotateTrust` cannot
  approve, reject, or widen visibility.

## Honest limitations

- Rules are English, small, and uncalibrated. Precision on real data is unknown.
- "contradicted" is a review prompt, not a decision.
- n=8 labeled pairs is too small for confidence intervals.
