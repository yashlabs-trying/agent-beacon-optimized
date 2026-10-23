# Phase 3B — Contradiction Detection Analysis

## 1. Phase 3A baseline and root causes of the false negatives

Phase 3A measured precision 1.000 / recall 0.250 on the 24-pair locked set. The
detector (`detectContradictions`) required: same `Project.ID`, same `Kind`, no
distinct explicit versions, **opposite non-zero polarity markers**, ≥ 2 shared
significant terms, and shared terms > half of the smaller term set. The nine
false negatives classified by root cause:

| Root cause | Pairs | Why missed |
|---|---|---|
| Cross-kind conflict | c07–c12 (6) | the `Kind` equality gate skipped the pair |
| Non-marker negation | c04 | "Always stop …" vs "Continue …" — "Continue" has no polarity marker |
| Non-marker negation | c05 | "Keep …" vs "Remove …" — neither verb is a marker |
| Non-marker negation | c06 | "Preserve …" vs "Never preserve …" — "Preserve" is not a positive marker, so the first side read as polarity 0 |

## 2. The fix (smallest justified change)

`cli/beacon/internal/learning/search_trust.go`:

1. **Opposing action verbs (default).** An explicit surface-form table of antonym
   pairs — `stop/continue`, `keep/remove`, `enable/disable`, `allow/deny`,
   `accept/reject`, `include/exclude` — flags a conflict when one memory uses one side
   and the other uses the opposite, even with no polarity marker.
2. **Negation asymmetry (default).** When exactly one side carries a negation token
   (`never`, `not`, `cannot`, `avoid`, …) and at least one side is a directive, the pair
   conflicts. Token-based, so "not" cannot fire inside "note".
3. **Cross-kind opt-in.** `TrustOptions.AllowCrossKind` (default `false`) removes the
   `Kind` equality gate for callers who want it.

Both new default signals run only after the existing scope, shared-term (≥ 2) and
overlap (> half) gates pass, so they only apply to memories that already concern the
same subject. The polarity-marker rule is unchanged.

### Why cross-kind is opt-in, not default

The datasets disagree on cross-kind semantics: the Phase 1 labelled matrix
(`trust_phase1_matrix_test.go`) labels different-`Kind` pairs as **non-conflicts** and
hard-asserts `fp == 0`; the Phase 2 adversarial matrix and the Phase 3 locked fixture
label them as **conflicts**. Making cross-kind default-on would break the Phase 1
contract and relax a safeguard. It is therefore an explicit opt-in.

## 3. Results

### Development set (new, 23 cases; not the locked fixture)

| Mode | TP | FP | FN | TN | Precision | Recall |
|---|---:|---:|---:|---:|---:|---:|
| default | 10 | 0 | 3 | 10 | **1.0000** | 0.7692 |
| cross-kind opt-in | 13 | 0 | 0 | 10 | **1.0000** | **1.0000** |

Categories: marker_conflict, antonym_conflict, negation_conflict, cross_kind_conflict,
distinct_version, cross_project, same_polarity, unrelated, insufficient_overlap. All
non-conflict categories are true negatives in both modes.

### Locked fixture (REGRESSION CHECK — already observed, not an independent holdout)

| Mode | Category | TP | FP | FN | TN |
|---|---|---:|---:|---:|---:|
| default | same_kind | 6 | 0 | 0 | 0 |
| default | cross_kind | 0 | 0 | 6 | 0 |
| default | distinct_version | 0 | 0 | 0 | 4 |
| default | cross_project | 0 | 0 | 0 | 4 |
| default | same_polarity | 0 | 0 | 0 | 2 |
| default | unrelated | 0 | 0 | 0 | 2 |
| default | **ALL** | **6** | **0** | **6** | **12** → P 1.000 / R 0.500 |
| cross-kind | **ALL** | **12** | **0** | **0** | **12** → P 1.000 / R **1.000** |

Per-pair: c01–c06 now TP (previously c04–c06 were FN); c07–c12 remain FN by default and
become TP under `AllowCrossKind`; c13–c24 remain TN.

### Cross-talk and existing suites (unchanged safeguards)

- Locked global cross-talk: **1 extra pair** (`c02-a` vs `c23-b`, sharing
  `[bearer expired request token]`) — identical to Phase 3A.
- Phase 2 adversarial matrix: precision 1.000, recall 0.758, cross-talk **37** — unchanged.
- Phase 1 labelled matrix: tp 10, fp 0, fn 0 — unchanged.
- `TestTrustContradictionPrecisionRecall`: 1.000 / 1.000 — unchanged.
- Cross-scope, unapproved-evidence, hostile-content, adversarial-retrieval: PASS.

> **Comparability warning.** Do not compare the Phase 2 65-case recall (0.758) with the
> 24-case locked recall (0.500) or the 23-case dev recall as if directly comparable.
> They are different datasets of different sizes and difficulty.

## 4. Remaining limitation

Default cross-kind recall is still 0. By design it stays off until a caller opts in.
The opt-in mode is validated at precision 1.000 on the dev set and the locked set, but
enabling it globally would relax the Phase 1 boundary contract, so it remains opt-in.

## 5. Reproduce

```
cd cli/beacon
go test ./internal/learning/ -run TestPhase3BContradictionDev -v -count=1
BEACON_PHASE3_CONTRADICTION=1 go test ./internal/learning/ -run TestPhase3BLockedContradictionRegression -v -count=1
BEACON_PHASE3_CONTRADICTION=1 go test ./internal/learning/ -run TestPhase3AContradictionLocked -v -count=1
go test ./internal/learning/ -run 'TestTrustContradictionPrecisionRecall|TestTrustPhase1LabeledScenarios|TestPhase2AdversarialContradictionMatrix|TestTrustNoCrossScopeLeakage' -v -count=1
```

Raw evidence: `RAW_EVIDENCE/contradiction_dev_and_locked.txt`,
`RAW_EVIDENCE/contradiction_locked_crosstalk.txt`, `RAW_EVIDENCE/security_trust.txt`.
