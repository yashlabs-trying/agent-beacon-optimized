# Phase 4D — SQLite and Concurrency

## 1. Scope

The storage engine was **not** rewritten. Phase 4D reuses the Phase 3C per-Store
`*sql.DB` pool and the Phase 3/4A concurrency and durability results, and confirms the new
selector adds no shared state (it is a pure function over retrieved memories).

## 2. Known limitation (retained, documented)

| Clients | Write p99 | Status |
|---:|---:|---|
| 1 / 4 / 8 | ≤135 ms | **PASS** (< 250 ms) |
| 16 | 744–795 ms | **FAIL** (≥250 ms) |

Root cause (Phase 3C): 16 concurrent `BEGIN IMMEDIATE` writers serialize on SQLite's
single write lock; concurrent readers amplify the tail. The ≤8-client operational budget
is retained. Phase 4D did **not** change this because no minimal, safe change preserves
durability/atomicity/crash-recovery (the brief forbids trading durability for latency).

## 3. What Phase 4D verified

- Full learning suite (includes crash durability, fault injection, concurrency,
  cross-scope leakage, hostile content, writer latency) → **ok 67.8 s**.
- `go test -race` concurrency/contention subset → **ok 42.9 s**.
- The selector is read-only and introduces no locks or shared state, so it cannot affect
  write concurrency (verified by race suite).

## 4. Narrow optimizations considered but not applied

- Shorter write transactions / batching: would alter transaction semantics; rejected
  without a durability-safe design.
- Disabling `wal_autocheckpoint`: tested in Phase 3C, did not help.
- Write scheduling: not implemented (no evidence it is safe under the current guarantees).

## 5. Concurrency at 32/64 clients

**BLOCKED** (host limits; not run).

## Verdict

≤8-client: **PASS**. 16-client p99: **FAIL** (documented ceiling, unchanged). Durability:
**PASS** (unchanged). 32/64: **BLOCKED**.
