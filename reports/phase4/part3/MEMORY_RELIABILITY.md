# Phase 4C — Memory Reliability Over Time (Benchmark D)

Bounded test of the context-selection layer under accumulating corrections.
Harness: `cli/beacon/internal/learning/phase4c_history_test.go`.
Evidence: `RAW_EVIDENCE/memory_history.txt`.

## 1. Correction accumulation (10 / 50 / 100 sessions)

Setup: one base fact (`The service port is 8080.`) plus N−1 verified corrections
(`The service port is now <v>.`), timestamps one day apart, all same project/kind.

| Sessions | Kept after selection | Superseded | Selection time |
|---:|---|---:|---:|
| 10 | `fact-9` (latest) | 9 | 0.88 ms |
| 50 | `fact-49` (latest) | 49 | 15.3 ms |
| 100 | `fact-99` (latest) | 99 | 44.7 ms |

Result: the layer retains **only** the currently applicable fact and drops the entire
superseded chain at every history size. **PASS** for correct-fact retention.

## 2. Untrusted correction must not override verified knowledge

Setup: verified `8080` + a newer **untrusted** `now 9`. Result: the verified fact is kept,
the untrusted correction is dropped. **PASS**.

## 3. Performance note (open concern)

The supersession scan is O(n²) pairwise. Selection time grows from 0.88 ms (n=10) to
44.7 ms (n=100) — roughly quadratic. At 1K memories this would be ~4.5 s per query if all
memories were candidates; in practice only the retrieved top-K are scanned, so the cost is
bounded by K², not corpus size. Still, the O(n²) inner loop is a scalability risk worth
optimising (e.g. index by shared subject terms).

## 4. Not measured here (honest scope)

- 1K/10K updates, repeated summarisation/compaction, index rebuilds, and process restarts
  as an *agent-outcome* study were **NOT RUN**; the storage-level crash/restart behaviour
  is covered by the reused Phase 3/4A suites (see `SCALABILITY_RESULTS.md`).
- Accuracy of agent outcomes over session progression (E3) is **BLOCKED** (compute).

## Verdict

Correct-fact retention and stale rejection over 10/50/100 sessions: **PASS**.
O(n²) selection: **CONDITIONAL** (bounded by K today; flagged).
