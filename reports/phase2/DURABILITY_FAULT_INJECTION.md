# Phase 2 — Durability and Crash-Injection

## Summary

A subprocess crash-injection harness was built and run against disposable SQLite
databases. **No acknowledged write was lost and no uncommitted write became
visible** in any tested crash scenario. `PRAGMA integrity_check` returned `ok`
after every crash, the derived index rebuilt after recovery, and repeated
recovery was idempotent. No critical blocker was found.

## Environment

Apple M5 / macOS 26.6.2 / go1.27.3 darwin-arm64. Worktree base `85e6ea1c`.
Store: `journal_mode=WAL`, `synchronous=2 (FULL)`, `busy_timeout=5000`,
`_txlock=immediate` (verified by reading the pragmas at runtime).

## Harness

`cli/beacon/internal/learning/phase2_crash_injection_test.go`.

- A nil-in-production fault hook (`faultHook`, called via `faultPoint`) is
  invoked at named stages of `PutMemory`, `rebuildMemoryIndexTx` and
  `ensureSchemaOnce`. In production the hook is nil, so each call site is a single
  nil comparison and **no fault-injection behaviour is reachable on a normal
  execution path** (`TestPhase2FaultHookIsNilInProduction` asserts this).
- The parent test spawns the test binary as a child
  (`-test.run=TestPhase2CrashChild`), which performs the write and is killed with
  `SIGKILL` at the target stage. SIGKILL runs no deferred rollback, a true process
  crash.
- The parent then reopens the database and checks integrity, acknowledged-write
  visibility, uncommitted-write invisibility, derived-index rebuild, and search
  correctness.

## Fault points and results

| Stage | Kill point | Acknowledged? | Uncommitted target visible? | Preseed survives? | Result |
|---|---|---|---|---|---|
| `tx_begin_pre` | before `Begin` | no | no | yes | PASS |
| `tx_begin_post` | after `Begin` | no | no | yes | PASS |
| `memory_insert` | after `INSERT memories` | no | no | yes | PASS |
| `fts_update` | after derived FTS upsert | no | no | yes | PASS |
| `generation_update` | after generation bump | no | no | yes | PASS |
| `pre_commit` | immediately before `Commit` | no | no | yes | PASS |
| `post_commit` | immediately after `Commit` | yes | yes (durable) | yes | PASS |
| `post_return` | after `PutMemory` returns nil | yes | yes (durable) | yes | PASS |

Every case: `PRAGMA integrity_check = ok`; the preseeded acknowledged write is
always present; the target write's visibility matches its acknowledgement; and a
post-recovery search returns exactly the committed set (the derived index rebuilds
from the authoritative rows).

### Other scenarios

| Test | Scenario | Result |
|---|---|---|
| `TestPhase2RepeatedRecovery` | crash at `pre_commit`, reopen, crash again at `memory_insert`, reopen, then commit | PASS |
| `TestPhase2CrashDuringIndexRebuild` | kill mid-rebuild of `memories_fts` | PASS — 40/40 authoritative rows intact, index rebuilds on next search |
| `TestPhase2CrashDuringMigration` | kill after authoritative schema creation, before derived schema | PASS — later open is idempotent and writes succeed |
| `TestPhase2CrashDuringSupersession` | kill mid-supersession | PASS — no dangling supersession pointer to an uncommitted memory |
| `TestPhase2CacheInvalidationUnderWrites` | cached result after a new commit | PASS — generation advance invalidates the cache; the new memory appears |

## Process crash vs power loss

This harness tests **process crash** (SIGKILL). It does **not** simulate power
loss, which can also lose data that a process has fsync'd but the storage device
has not persisted, and can tear writes below the page cache. The relevant
configuration evidence is that the store runs WAL with `synchronous=FULL`, which
fsyncs the WAL on every commit; that is the durable setting and is a necessary
(but not sufficient) condition for power-loss durability. **Power-loss durability
is NOT TESTED** and should not be claimed from these results.

## ANN index persistence

There is no ANN persistence path: ANN indexes are in-memory derived state, keyed
by a durable generation counter and rebuilt from `memory_embeddings` on demand
(see `SCALING_RESULTS.md`). A crash therefore cannot corrupt a persisted ANN
index; the only durable state is the SQLite database, which is covered above.

## Reproduce

```
BEACON_PHASE2_STRESS=  go test ./internal/learning/ \
  -run 'TestPhase2Crash|TestPhase2RepeatedRecovery|TestPhase2FaultHook' -v -count=1
```
