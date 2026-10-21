# Reliability report

Status: inherited v2 coverage verified; some fault classes not tested.

## Verified (tests pass; concurrency tests run under `-race`)

| property | test | result |
|---|---|---|
| concurrent readers + writers | `TestSearchConcurrentReadersAndWriters` | no race, no leak, all acknowledged writes visible |
| concurrent supersede + read | `TestSearchConcurrentSupersedeAndRead` | superseded rows never returned |
| cache invalidation across store instances | `TestSearchCrossStoreVisibility` | write via one Store visible to another |
| stale cache after mutation | `TestSearchCacheInvalidatedByWrite` / `...BySupersede` | pass |
| index deletion / corruption fallback | `TestSearchIndexFallsBackWhenIndexTableDropped`, `TestSearchRecoversAfterIndexDrop` | correct result; index rebuilt and ready |
| repeated rebuild | `TestSearchIndexRebuildIsIdempotent` | one index row per memory |
| SQLite busy handling | `_txlock=immediate` + `_busy_timeout` | no `SQLITE_BUSY` write loss under load |
| semantic incremental update | `TestSemanticIncrementalUpdate` | unchanged content not re-embedded |
| cross-scope isolation | `TestTrustNoCrossScopeLeakage` | zero leakage |
| generation-safe cache | generation read before compute | stale results never served |

Full learning suite and `-race` both pass (race run 520 s).

## Measured

- Incremental write: 373 µs (baseline) → 608 µs (indexed) per write.
- WAL after 5000 writes: 0 B (per-call connection close checkpoints the WAL).
- Acknowledged write durability under derived-index loss: verified.

## NOT verified (honest gaps)

- **Process termination mid-write.** Not injected; relies on SQLite atomic commit.
- **Memory pressure / OOM.** Not exercised.
- **Sustained soak and p99 under load.** Not measured.
- **Cache hit rate instrumentation.** Not added.
- **Semantic/ANN fault injection** (embedding-table deletion, ANN rebuild failure).
- **Interrupted migration.** Not tested.
- **Crash during ANN index build.** Not tested (in-memory only).

## Root causes of any observed failures

None outstanding in the current suite. Two defects were found and fixed earlier
in this line of work (stale-cache TOCTOU; write failure on derived-index loss);
both have regression tests.
