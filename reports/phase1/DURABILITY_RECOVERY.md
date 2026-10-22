# Phase 1 — Durability and Recovery

## Verified (WSL2)

| Scenario | Test | Result |
|---|---|---|
| Derived index dropped | `TestSearchRecoversAfterIndexDrop` | write succeeds with index gone; next query correct; index rebuilt, `index_ready=1` |
| Index table deleted, query | `TestSearchIndexFallsBackWhenIndexTableDropped` | correct result via scan |
| Repeated rebuild | `TestSearchIndexRebuildIsIdempotent` | one index row per memory |
| Write durability under derived-index loss | PutMemory marks index not-ready and still commits | PASS |
| Generation consistency | generation read before compute; cache keyed on it | PASS |
| Restart/reopen (fresh connections) | cross-store test | PASS |

SQLite is WAL with `_txlock=immediate` and `busy_timeout`; the store opens/closes
a connection per call, so the WAL is checkpointed on close (0 B after a write
burst). Durability here is **process-crash durability**, not power-loss.

## NOT TESTED (explicit gaps)

- **Crash-mid-transaction** (subprocess kill at: before tx, after row insert,
  during index update, during generation update, before/after commit) — no fault
  injection harness was built. This is the main Phase 2 durability task.
- **Interrupted migration**.
- **Power-loss durability** (not claimed; requires storage-level testing).

## Root cause of any observed failures

None in the tested set. Two earlier defects (stale-cache TOCTOU; write failure on
derived-index loss) were found and fixed in V3/V5 with regression tests.

## Pass condition

No lost acknowledged writes or authoritative corruption in the **tested**
scenarios: **CONDITIONAL PASS** (index-loss recovery verified; crash injection not
run).
