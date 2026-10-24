# Trust and evidence benchmarks (Component 3)

Status: **PARTIAL**. Deterministic validators are implemented and measured;
model-based adjudication and calibrated confidence are not implemented.

## Setup

- Machine: WSL2 Ubuntu, 8 vCPU, i5-1155G7, Go 1.27.0.
- Tests: `TestTrustEvidenceStatus`, `TestTrustContradictionPrecisionRecall`,
  `TestTrustNoCrossScopeLeakage`.
- Pure functions over `[]LearningMemoryV1`; no store mutation, no model call.

## Evidence-status accuracy

Five labeled memories, `MaxAgeDays=180`:

| memory | expected | got |
|---|---|---|
| recent + trace ID | verified | verified |
| no evidence | unverified | unverified |
| evidence with blank trace ID | unverified | unverified |
| evidence but 400 days old | stale | stale |
| superseded but evidenced | verified (evidence ≠ approval) | verified |

Accuracy 5/5. Supersession and approval are deliberately kept separate from
evidence status.

## Contradiction detection

8 labeled pairs (4 true conflicts with opposite polarity, 4 decoys with shared
terms and the same polarity), each in its own kind so cross-pair detection
cannot occur.

| metric | value |
|---|---|
| true pairs | 4 |
| detected pairs | 4 |
| true positive | 4 |
| false positive | 0 |
| false negative | 0 |
| precision | 1.00 |
| recall | 1.00 |

The rule is deterministic: same project and kind, ≥2 shared significant terms,
and opposite directive polarity (strong markers such as "always"/"never" take
precedence over weak ones such as "prefer"/"avoid").

## Cross-scope leakage

2 projects × 30 memories with identical bodies. Queries scoped to `proj-a` and
`proj-b` returned **zero** memories from the other project, on both the lexical
and hybrid paths.

## Honest limitations

- These are rules, not calibrated probabilities. A "contradicted" label is a
  prompt for human review, not a decision, and its precision on real data is
  unknown; n=8 labeled pairs is small.
- The polarity lexicon is English and small; it will miss conflicts phrased
  without its markers and may flag coincidental term overlap.
- No stale-memory retrieval rate or false-rejection rate was measured on a large
  corpus; the evidence-status check is per-memory and deterministic.
- Model-based adjudication is **not implemented**; there is no confidence
  threshold because there is no model.
- Approval, provenance and project isolation are unchanged and were not
  weakened; `AnnotateTrust` cannot approve, reject, or widen visibility.

## Reproduction

```bash
cd cli/beacon
go test -run 'TestTrust' -v ./internal/learning/
```
