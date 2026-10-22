# Phase 1 — Trust, Provenance and Security (22 October 2026)

On WSL2 and Mac, `go test -run 'TestTrustPhase1LabeledScenarios|TestAdversarialMemoryRetrieval' -v ./internal/learning` passed. `TestTrustPhase1LabeledScenarios` contains 20 provenance/status cases and 30 contradiction pairs, covering blank and valid evidence, staleness, supersession, project/kind boundaries, same and distinct software versions, and non-opposing directives. The 30 pairs gave TP=10, FP=0, FN=0 (precision=recall=1.000 **on this set**). Before repair, five distinct-version pairs were false positives (precision 0.667); `distinctExplicitVersions` now suppresses a conflict only when both applicability fields name one distinct, unambiguous version. Ambiguous ranges remain review candidates. The prior n=8 contradiction result remains recorded below.

`TestAdversarialMemoryRetrieval` used only disposable projects, fake secrets, and mock directives. Cases included fake system prompts, tool-use manipulation, synthetic exfiltration text, forged approval claims, SQL-like strings, 256 KiB content, malformed metadata, and cross-project retrieval. The tested storage/retrieval layer treated these as data, did not auto-approve them, and did not return cross-project memories. Stored content was **not** given instruction authority by the retrieval API. The agent harness used ordinary task notes; it did not adversarially inject those memories into a live model. Therefore agent-level prompt-injection resistance remains untested and this is a **conditional** security pass.

These labels are synthetic and predominantly English. Precision/recall on real reviews and ambiguous policy language are unknown. See `trust_phase1_matrix_test.go` and `security_smoke_test.go` for exact fixtures and expected outcomes.

## Prior Phase 1 report (preserved below)

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
