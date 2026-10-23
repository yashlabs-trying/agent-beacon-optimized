# Phase 3A — Contradiction Detection and Trust (Workstream C)

**Status: precision gate PASS (1.000); recall gate FAIL (0.250). Zero project/version
boundary violations. Advisory-only behaviour preserved.**

## 1. Method

- Fixture: `reports/phase3/heldout/contradiction_v1.json` (locked, hash `B3B4CAC0…8E84`),
  24 pairs: 12 labelled conflict, 12 non-conflict.
- Each pair evaluated **in isolation** (matching the fixture's pairwise design) via the
  production detector `detectContradictions(memories, TrustOptions{ContradictionTerms:2})`.
  Labels were read from the fixture and **never changed after seeing results**.
- Mapping: `project_a/project_b → Project.ID`, `kind_a/kind_b → Kind`,
  `version_a/version_b → Applicability`, `a/b → Body`.
- A second pass loads all 48 memories together to measure cross-pair "cross-talk".

## 2. Per-pair verdicts

| Pair | Category | Label | Detected | Verdict |
|---|---|---|---|---|
| c01 | same_kind | conflict | yes | TP |
| c02 | same_kind | conflict | yes | TP |
| c03 | same_kind | conflict | yes | TP |
| c04 | same_kind | conflict | no | **FN** |
| c05 | same_kind | conflict | no | **FN** |
| c06 | same_kind | conflict | no | **FN** |
| c07–c12 | cross_kind | conflict | no | **FN** (×6) |
| c13–c16 | distinct_version | non-conflict | no | TN (×4) |
| c17–c20 | cross_project | non-conflict | no | TN (×4) |
| c21–c22 | same_polarity | non-conflict | no | TN (×2) |
| c23–c24 | unrelated | non-conflict | no | TN (×2) |

## 3. Aggregate metrics (n = 24)

| Category | TP | FP | FN | TN | Precision | Recall | F1 |
|---|---:|---:|---:|---:|---:|---:|---:|
| same_kind | 3 | 0 | 3 | 0 | 1.0000 | 0.5000 | 0.6667 |
| cross_kind | 0 | 0 | 6 | 0 | — | 0.0000 | 0.0000 |
| distinct_version | 0 | 0 | 0 | 4 | — | — | — |
| cross_project | 0 | 0 | 0 | 4 | — | — | — |
| same_polarity | 0 | 0 | 0 | 2 | — | — | — |
| unrelated | 0 | 0 | 0 | 2 | — | — | — |
| **ALL** | **3** | **0** | **9** | **12** | **1.0000** | **0.2500** | **0.4000** |

- **Precision 1.000** — no false contradictions on any of the 12 non-conflict pairs.
- **Recall 0.250** — 9 of 12 labelled conflicts missed.

### Why the same_kind conflicts were missed

The detector requires opposite **explicit polarity markers** (`always/must/never/do not/…`).
Three labelled conflicts use other wording:

- c04 `Continue distributed writes …` — "Continue" is not a negative marker ⇒ polarity 0.
- c05 `Keep …` vs `Remove …` — neither is a marker ⇒ both polarity 0.
- c06 `Preserve …` vs `Never preserve …` — "Preserve" is not a positive marker ⇒ first
  side polarity 0, so the pair is skipped.

### Why the cross_kind conflicts were missed

`detectContradictions` requires **identical `Kind`** on both sides (by design). All six
cross_kind conflicts (c07–c12) have different `Kind` values, so they are skipped. This is
the dominant recall loss and matches the Phase 2 "cross-kind missed" limitation.

## 4. Cross-scope, version, and cross-talk

- **Project boundary:** all 4 `cross_project` pairs are TN — the `Project.ID` guard holds.
- **Version boundary:** all 4 `distinct_version` pairs are TN — `distinctExplicitVersions`
  correctly suppresses `v3/v4`, `v7/v8`, `version 2.1/version 3.0`, `version 1/version 2`.
  No false contradiction involves an explicit version change.
- **Global cross-talk:** loading all 48 memories, `detectContradictions` returned **4**
  pairs: 3 matched labelled conflicts and **1 extra cross-pair pair** —
  `c02-a` ("Always refresh the expired bearer token …") vs `c23-b` ("Never reuse an expired
  bearer token …"), same project (`alpha`) and same kind (`gotcha`), sharing
  `[bearer, expired, request, token]`. This is a genuine global false positive that a
  single pair evaluation cannot see.

> **Comparability warning.** The Phase 2 adversarial matrix (65 cases) reported recall
> 0.758 with 37 cross-talk pairs; this locked set (24 pairs) reports 0.250 with 1
> cross-talk pair. These are **different datasets of different sizes** and must not be
> compared as if directly comparable.

## 5. Trust boundaries and hostile content

| Check | Result |
|---|---|
| Cross-scope leakage (lexical, hybrid exact, hybrid ANN, path traversal) | **PASS** |
| Unapproved evidence invisible; approved visible | **PASS** |
| Hostile stored content (SQL injection, path traversal, 200 KB oversized, unicode-bidi) | **PASS** — table survives; oversized round-trips exactly |
| Adversarial contradiction matrix (65 cases) | precision 1.000, recall 0.758, FPR 0.000, cross-talk 37 |
| Trust annotation latency (n=500) | 5.01 ms total, 10.0 µs/item |
| Phase 1 labelled scenarios (20 status + 30 pair cases) | TP 10, FP 0, FN 0 |
| Contradiction handling is advisory only | **PASS** — no automatic approval or deletion; the layer never approves |

**No hostile memory content bypassed approval or the trust boundary in these synthetic
tests.** This does not establish real-world prompt-injection resistance.

## 6. Reproduce

```
cd cli/beacon
BEACON_PHASE3_CONTRADICTION=1 go test ./internal/learning/ -run TestPhase3AContradictionLocked -v -count=1
go test ./internal/learning/ -run 'TestTrustContradictionPrecisionRecall|TestTrustPhase1LabeledScenarios|TestPhase2AdversarialContradictionMatrix|TestTrustNoCrossScopeLeakage|TestPhase2CrossScopeLeakageAllPaths|TestPhase2UnapprovedEvidenceNotVisible|TestPhase2HostileMemoryContent|TestAdversarialMemoryRetrieval' -v -count=1
```

Raw evidence: `RAW_EVIDENCE/contradiction_locked.txt`, `RAW_EVIDENCE/security_trust.txt`.
