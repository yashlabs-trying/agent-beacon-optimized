# Phase 3C — Store Pooling Design

## 1. Problem (Phase 3B diagnosis)

Every store operation called `s.db()`, which did `sql.Open(...)` and was followed by
`defer db.Close()` — a fresh `*sql.DB` pool (and physical SQLite connection) per read or
write. Phase 3B isolated this as the dominant cost: a shared pool gave 16 concurrent
writers p99 ≈ 214 µs versus the store's ≈ 940 ms end-to-end tail, while fsync and index
rebuilds made no difference.

## 2. Change

`Store` now owns exactly one `*sql.DB` for its lifetime.

```go
type Store struct {
    dbPath   string
    embedder Embedder
    writeCheckpoint func(string)

    mu     sync.Mutex
    dbPool *sql.DB
    closed bool
}
```

- **Lazy, race-free creation.** `db()` takes `s.mu`, returns `ErrStoreClosed` if closed,
  and otherwise calls `openLocked()` once. Concurrent callers receive the same pool.
- **`openLocked()`** creates the pool, applies `SetMaxIdleConns(2)` and
  `SetConnMaxIdleTime(30s)` (so an abandoned pool cannot hold descriptors forever),
  runs `ensureSchemaOnce`, and on any failure closes the pool before returning — no
  handle leak on a failed open.
- **`Close()`** is idempotent; it marks the store closed and closes the pool. Subsequent
  `db()` calls return `ErrStoreClosed`, and `PutMemory` / `ListMemories` propagate it.
- **Path normalization** moved from `db()` into `Open`/`OpenSemantic`, so `db()` no
  longer mutates `s.dbPath` (removing a latent data race with readers of `Path()`).
- **11 call sites** in `store.go` and `search_semantic.go` dropped `defer db.Close()`.

### Schema initialization rework

`schemaInitialized` remains a process-wide `sync.Map` keyed by path (so multiple Stores
and subprocesses on one file coordinate), but `ensureSchemaOnce` now double-checks under
a new `schemaInitMu`:

```go
func (s *Store) ensureSchemaOnce(db *sql.DB) error {
    if _, ok := schemaInitialized.Load(s.dbPath); ok { return nil }
    schemaInitMu.Lock(); defer schemaInitMu.Unlock()
    if _, ok := schemaInitialized.Load(s.dbPath); ok { return nil }
    // PRAGMA journal_mode=WAL; ensureSchema; faultPoint("migration_post_schema");
    // ensureSearchSchema; schemaInitialized.Store(...)
}
```

This makes concurrent initialization idempotent and race-free, and guarantees no caller
observes a partially initialized schema.

`markIndexUnhealthy` previously relied on the next `db()` call to re-run
`ensureSchemaOnce`. Because the pool is now reused, it re-runs initialization directly:

```go
func (s *Store) markIndexUnhealthy(db *sql.DB) {
    _, _ = db.Exec(`UPDATE memory_index_meta SET value = '0' WHERE key = 'index_ready'`)
    indexReadyCache.Delete(s.dbPath)
    schemaInitialized.Delete(s.dbPath)
    _ = s.ensureSchemaOnce(db)   // recreate derived tables if a table is missing
}
```

`PutMemory`'s index-failure path now calls `s.markIndexUnhealthy(db)` instead of only
deleting the caches.

## 3. Behaviour preserved

- Public API unchanged (`Open`, `OpenSemantic`, `PutMemory`, `ListMemories`,
  `SearchHybrid`, …); only `Close` and `ErrStoreClosed` are added.
- Transaction boundaries, WAL mode, `busy_timeout(30000)`, `_txlock=immediate`, durable
  generation counter, and the derived-index lifecycle are unchanged.
- Crash/durability semantics unchanged: the fault-injection seam (`faultHook`,
  `writeCheckpoint`) and all write-boundary fault points are untouched; the full crash
  and durability suite passes.
- No global pool was introduced; each `Store` owns its pool.

## 4. Test ownership audit

Tests that called `store.db()` on a Store they continued to use no longer close it
(the Store owns the pool). Changed: `search_test.go` (`seedMemories`, `oracleScan`, two
index-drop tests), `store_test.go` (`setMemoryUpdatedAt`), `search_reliability_test.go`,
`ann_test.go`, `neural_test.go`, `neural_extended_test.go`,
`concurrency_phase1_matrix_test.go`, `phase2_integration_test.go`,
`phase2_token_ranking_test.go`, `phase2_trust_adversarial_test.go`,
`phase2_concurrency_test.go` (churn), `phase3b_writer_test.go` (churn, microbenchmarks).

`openRaw` still creates a throwaway `Store` and returns its pool; callers close that
pool, which is correct because the `Store` is discarded.

## 5. Lifecycle regression tests (`phase3c_pool_test.go`)

| Test | Asserts |
|---|---|
| `TestStoreReusesOnePool` | 50 `db()` calls return the identical pool; reads work |
| `TestStoreCloseIsIdempotentAndBlocksUse` | double `Close` is a no-op; `db()`/`PutMemory`/`ListMemories` return `ErrStoreClosed` |
| `TestMultipleStoresIndependent` | two Stores use distinct pools; closing one leaves the other usable |
| `TestConcurrentStoreInitRaceFree` | 32 goroutines race lazy init; all get the same pool (race-clean) |
| `TestConcurrentStoresSamePathInitRaceFree` | 12 Stores on one path init concurrently; a write through one is visible in another |
| `TestFailedInitializationDoesNotLeakPool` | a directory path fails to open, no pool retained, repeat fails the same way |
| `TestConcurrentReadsWritesThenClose` | 8 readers + 4 writers, then a clean `Close` |

All pass, including under `-race`.

## 6. Residual notes

- With a persistent pool the WAL is no longer checkpointed on every operation's
  connection close, so `-wal` grows to the auto-checkpoint threshold (~4 MB) and up to
  ~50 MB under sustained concurrent readers in the raw probe. This is normal WAL
  behaviour, not a correctness change; the database remains durable and
  `PRAGMA integrity_check` passes.
- The pool keeps up to two idle connections per Store. Long-running consumers
  (MCP/dashboard) benefit; short-lived CLI invocations open one pool and exit.
