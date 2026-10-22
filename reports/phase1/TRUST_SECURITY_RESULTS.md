# Phase 1 — Trust, Provenance and Security

## Trust (deterministic)

`TestTrustEvidenceStatus` (5 labeled memories): 5/5 correct
(verified / unverified / unverified-on-blank-source / stale / superseded-still-verified).

`TestTrustContradictionPrecisionRecall` (8 labeled pairs, 4 true conflicts, 4
decoys): precision **1.00**, recall **1.00**.

`TestTrustNoCrossScopeLeakage` (2 projects × 30 identical memories, lexical +
hybrid): **zero** cross-project leakage.

Approval semantics are unchanged: `AnnotateTrust` cannot approve, reject, or widen
visibility. No heuristic auto-approval exists.

## Security smoke

- Scope isolation: **verified** (above) — retrieval is SQL-scoped by `project_id`;
  ANN graphs are per-project; hydration filters supersession.
- Prompt-injection / hostile memory content / fake provenance / SQL-like strings /
  oversized input / path traversal / canary secrets: **NOT TESTED** — no injection
  harness was built in Phase 1. This is a real gap, not a pass.
- Storage-level security (scope, approval) is separated from agent-level
  prompt-injection resilience; the latter is unverified.

## Pass condition

No unauthorized cross-scope results and no critical trust bypass in the defined
tests: **PASS** for the defined (small) tests. Security smoke is **NOT TESTED**.

## Limitations

- n=8 contradiction pairs and n=5 status cases are far below the 50-scenario
  target; precision on real data is unknown.
- Rules are English and uncalibrated; "contradicted" is a review prompt, not a
  decision.
