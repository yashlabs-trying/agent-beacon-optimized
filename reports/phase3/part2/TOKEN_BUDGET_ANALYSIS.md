# Phase 3B — Token-Budget Analysis

## 1. Reproduction (existing Phase 2 harness, unchanged)

`TestPhase2TokenSelectionStrategies`, 48 held-out queries, 144 finding-slots
(48 × {primary, supporting-1, supporting-2}):

| Strategy | Tokens | Relevant retained | Redundancy | µs/case |
|---|---:|---:|---:|---:|
| `fixed_top5` (unbudgeted) | 5,674 | 109/144 | 0.569 | 0.0 |
| `token_budget` (current) | 3,840 | 106/144 | 0.574 | 15.1 |
| `evidence_dedup` | 5,563 | **112/144** | **0.525** | 4.0 |
| `trust_aware` | 5,670 | 109/144 | 0.581 | 2,052.5 |
| `diversity_mmr` | 5,674 | 109/144 | 0.569 | 8,091.5 |

Reproduces Phase 3A exactly: 106/144 at 3,840 tokens vs 109/144 at 5,674 tokens.

## 2. Which findings are lost, and are they important?

`TestPhase3BTokenLostFindings` classifies the 144 slots by importance:

| Group | Kept by top5 | Kept by budget | Kept by dedup |
|---|---:|---:|---:|
| primary (gold/dup) | 48 | **48** | 48 |
| supporting-1 (related) | 24 | 23 | 26 |
| supporting-2 (related2) | 37 | 35 | 38 |

Losses (`top5` retains, `budget` drops): **gold 0, related 1, related2 2**.

- The **primary finding is never lost** by the budgeted selector (48/48).
- The three lost findings are all **supporting** findings: `topic=11 related`,
  `topic=17 related2`, `topic=23 related2`.
- The loss is therefore not on the highest-value finding, but it is still a genuine
  evidence loss, so the Phase 3A "retain every top-five finding" gate still **FAILS**.

## 3. Evidence preservation

`TestPhase3BTokenLostFindings` also checks provenance coverage of the budgeted
selection: of 181 kept items, **0 lost provenance** that they carried in the ranked
list. Two kept items never had usable evidence at all (unverified distractor memories);
truncation did not strip evidence from any item that had it.

## 4. Alternative policy (development set)

`evidence_dedup` (drop results whose joined `TraceID` set was already seen) is the best
evidence-preserving policy: it retains the **most** findings (112/144), has the lowest
redundancy (0.525), and is cheap (4 µs/case). `TestPhase3BEvidenceDedupPreservesFindings`
asserts it retains at least as many findings as unbudgeted top-five (112 ≥ 109: PASS). It
saves only ~2 % of tokens, so it is not a token-saving default either.

## 5. Decision

- Token budgeting stays **opt-in** (it fails the retain-all gate by 3 supporting findings).
- The primary finding is safe (48/48) and provenance is never stripped.
- If a token-saving default is ever wanted, the measured path is evidence-preserving
  deduplication combined with a smaller budget, validated on real tasks in a later phase.

## 6. Reproduce

```
cd cli/beacon
go test ./internal/learning/ -run 'TestPhase2TokenSelectionStrategies|TestPhase3BTokenLostFindings|TestPhase3BEvidenceDedupPreservesFindings' -v -count=1
```

Raw evidence: `RAW_EVIDENCE/token_analysis.txt`.
