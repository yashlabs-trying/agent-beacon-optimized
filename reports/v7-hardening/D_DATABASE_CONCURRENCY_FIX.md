# Beacon V7 hardening — Phase 1: Database Concurrency Fix

Status: **FIXED on a local filesystem. 16-client write p99 634 ms → ~48–50 ms (≤250 ms
target met), 0 errors/busy, durability preserved.** On OneDrive the ceiling is the
filesystem (~1 s/commit), not the code.

## 1. Root cause (Step A — profiled via the writer harness)

- The pool uses `_txlock=immediate` + `busy_timeout(30000)`. With 16 concurrent writers
  each holding its own connection, every writer attempts `BEGIN IMMEDIATE` and then
  **busy-polls SQLite's single write lock**. The losers spin in the busy handler until
  the winner commits, producing a long, super-linear tail (16-client p99 634 ms vs
  8-client 130 ms) while throughput collapses.
- The tail is **not** per-op pool churn (fixed in Phase 3C) and **not** the index work;
  it is lock convoying among 16 OS threads.

## 2. Change (Step B — smallest proven fix)

`cli/beacon/internal/learning/store.go`:

- Added `writeMu sync.Mutex` to `Store`.
- `PutMemory` acquires `writeMu` around the write transaction.

Only one goroutine holds a SQLite write transaction at a time; the others park on a Go
mutex (cheap) instead of spinning in the SQLite busy handler. **Durability is unchanged**:
every write still commits under WAL with `synchronous` untouched; `_txlock=immediate`
and the 30 s busy_timeout remain (now rarely exercised in-process). Readers are unaffected
(WAL readers do not block writers).

## 3. Result (ext4, mixed workload, 8 s)

| Clients | write p50 | write p95 | write p99 | read p99 | errors/busy |
|---:|---:|---:|---:|---:|---:|
| 1 | 1.1 ms | 2.7 ms | 4.9 ms | 0.57 ms | 0 / 0 |
| 4 | 5.6 ms | 12.4 ms | 21.8 ms | 0.81 ms | 0 / 0 |
| 8 | 13.4 ms | 25.7 ms | 33.3 ms | 0.83 ms | 0 / 0 |
| **16** | 27.1 ms | 40.7 ms | **48.1 ms** | 0.82 ms | 0 / 0 |
| 32 | 51.0 ms | 83.9 ms | 94.3 ms | 0.77 ms | 0 / 0 |

| Before | After |
|---|---|
| 16-client write p99 **634 ms** | **48 ms** (rep 2: 50 ms) |

**Write-heavy, 16 clients:** write p99 **16.4 ms**, throughput **16,091 writes / 8 s
≈ 2,011 writes/s**, 0 errors/busy.

The median rises (2.9 ms → 27 ms) because writes now queue; the tail collapses. Net
throughput improved (16-client writes 3,319 → ~4,475 / 8 s).

## 4. Acceptance gate

**16-client write p99 ≤ 250 ms: PASS (48–50 ms on ext4), 0 lost acknowledged writes.**
Durability suite (crash recovery, bounded-write contention, concurrent read/write) passes.

## 5. Filesystem caveat (must be stated)

On the OneDrive mount (`/mnt/c`) the same fixed code shows 16-client write p99 ≈ 1.34 s
because each commit fsyncs with ~1 s latency. The production database must live on a
local filesystem (e.g. `~/.local/share/...`), not a synced/network mount. This is a
deployment constraint, reported, not silently changed.

## 6. Remaining limit

The write tail now scales ~linearly with client count (serialized writes): 32 clients →
94 ms. This is the expected cost of a single-writer store and stays within budget through
32 clients. 64 clients: **NOT TESTED** (hardware).

## 7. Verification & commit

- `go build ./...` pass; `TestPutMemoryProcessCrashRecovery`,
  `TestConcurrentReadsWritesThenClose`, `TestSearchConcurrentReadersAndWriters`,
  `TestPutMemoryWaitsForBoundedWriteContention` pass.
- Committed locally on `codex/beacon-v7-hardening`.
