# Reliability benchmarks (Component 4)

Status: **PARTIAL**. The v2 reliability work is preserved and covered by tests;
v3 adds semantic/trust state without regressing it. Some fault classes remain
untested.

## Covered (tests, all pass; the concurrency tests run under `-race`)

| fault class | test | result |
|---|---|---|
| concurrent readers + writers | `TestSearchConcurrentReadersAndWriters` | no race, no leak, all acknowledged writes visible |
| concurrent supersede + read | `TestSearchConcurrentSupersedeAndRead` | superseded rows never returned |
| cache invalidation across store instances | `TestSearchCrossStoreVisibility` | write via one Store visible to another next query |
| stale cache after mutation | `TestSearchCacheInvalidatedByWrite` / `...BySupersede` | pass |
| index corruption / deletion | `TestSearchIndexFallsBackWhenIndexTableDropped`, `TestSearchRecoversAfterIndexDrop` | correct result; index rebuilt and ready |
| repeated rebuilds | `TestSearchIndexRebuildIsIdempotent` | one index row per memory |
| SQLite busy conditions | `_txlock=immediate` + `_busy_timeout`; concurrency tests | no `SQLITE_BUSY` loss under load |
| restart recovery | cross-store test (fresh connections) | generation check prevents stale reads |
| semantic incremental update | `TestSemanticIncrementalUpdate` | unchanged content not re-embedded; changed content refreshed |
| cross-scope isolation | `TestTrustNoCrossScopeLeakage` | zero leakage |

## Measured (from `SEARCH_BENCHMARKS.md`, v2)

- Incremental write cost: 373 µs/write baseline, 608 µs/write with the index.
- WAL after 5000 writes: 0 B (per-call connection close checkpoints the WAL).
- Acknowledged write durability: verified under derived-index loss
  (`TestSearchRecoversAfterIndexDrop`).

## Not covered (honest gaps)

- **Process termination mid-transaction.** Not injected. SQLite's own atomic
  commit is relied on; no kill-during-write test was run.
- **Memory pressure / OOM.** Not exercised.
- **Long-running mixed workload soak.** Not run.
- **Cache hit rate under a realistic workload.** Not instrumented.
- **p99 read/write latency under sustained concurrency.** The concurrency tests
  assert correctness, not tail latency.
- **Semantic layer fault injection.** Embedding-table deletion is not tested
  (embeddings are derived and would be rebuilt by `RebuildEmbeddings`).

## Reproduction

```bash
cd cli/beacon
go test ./internal/learning/...            # correctness
go test -race ./internal/learning/...      # concurrency
```
