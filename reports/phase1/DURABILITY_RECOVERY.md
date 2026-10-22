# Phase 1 — Durability and Recovery (22 October 2026)

`go test -run 'TestPutMemoryProcessCrashRecovery|TestPutMemoryWaitsForBoundedWriteContention' -v ./internal/learning` passed on WSL2 and Mac after repair. A subprocess opened a disposable SQLite database and exited at six write checkpoints: after transaction begin, authoritative row, FTS/index update, generation update, before commit, and after commit. On restart, `PRAGMA integrity_check`, authoritative row visibility, index/retrieval behavior, and generation consistency matched the commit boundary. Pre-commit exits rolled back; after-commit exit preserved the committed row, although the child caller did not receive an acknowledgement. The checkpoint callback is unexported and unset in normal operation. Existing index-drop, rebuild, cross-store, and cache tests also passed.

A separate real write-lock holder kept an immediate SQLite transaction open for 11 seconds. Before repair, the 5-second busy timeout returned `SQLITE_BUSY`, despite a write becoming possible shortly afterward. Raising the DSN `_pragma` and `_busy_timeout` to 30 seconds let the waiting PutMemory complete; reopening confirmed its acknowledged write. This bounds local contention, but is not a throughput or arbitrary-duration starvation guarantee.

Process exit is **not** a power cut. This harness does not simulate torn storage writes, interrupted schema migration, a kill inside SQLite's commit implementation, or a process exit specifically during index rebuild/cache invalidation. Those remain Phase 2 reliability work. Earlier claims and tests are retained below as historical provenance.

## Prior Phase 1 report (preserved below)

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
