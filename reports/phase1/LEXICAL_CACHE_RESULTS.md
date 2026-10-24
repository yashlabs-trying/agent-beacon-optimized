# Phase 1 — Lexical and Cache Results (22 October 2026)

V6 base `85e6ea1c`; original Beacon baseline `99c70908`. On WSL2, the learning package passed `go test -count=1 ./internal/learning`, including `TestSearchDifferentialRandomized` (10,000 randomized queries), `TestSearchMatchesScanAcrossCases`, and `TestSearchDifferentialEdgeCases`. These cover FTS5/indexed-versus-scan equivalence, common/rare/short/Unicode/punctuation/empty terms, pagination, sorting, updates, scope, and supersession. Cache generation, cross-store visibility, index drop and rebuild, and eviction tests passed. The concurrency matrix in `CONCURRENCY_RESULTS.md` exercised cache invalidation under overlapping clients.

The same added `BenchmarkPhase1Lexical` file ran in the detached baseline and V6 worktrees on the **same Apple M5 Mac**: `go test -run '^$' -bench '^BenchmarkPhase1Lexical$' -benchtime=30x -count=1 -timeout 30m ./internal/learning`. Each store contained 100, 1K, 5K, or 10K deterministic memories, one rare marker per 100 records, and a no-match query. Setup was excluded from the timer. Raw benchmark output, including bytes and allocations, is in `artifacts/lexical_baseline.txt` and `artifacts/lexical_v6.txt`.

| Memories | Query | Baseline ms/op | V6 ms/op | Speedup |
|---:|---|---:|---:|---:|
| 100 | rare | 0.508 | 0.304 | 1.67× |
| 100 | no match | 0.453 | 0.273 | 1.66× |
| 1,000 | rare | 1.147 | 0.307 | 3.74× |
| 1,000 | no match | 1.902 | 0.344 | 5.53× |
| 5,000 | rare | 1.135 | 0.341 | 3.33× |
| 5,000 | no match | 8.530 | 0.333 | 25.60× |
| 10,000 | rare | 1.130 | 0.316 | 3.57× |
| 10,000 | no match | 16.642 | 0.306 | 54.45× |

The rare query matches about 1% of records, so its baseline cost stops rising after the result limit. These are single-process microbenchmarks with 30 timed iterations per cell, not end-to-end agent latency. The earlier 1M result (3,263 ms to 0.61 ms for no match) belongs to the prior GPU-pod/V6 report and was not rerun in this Phase 1 effort.

## Prior Phase 1 report (preserved below)

# Phase 1 — Lexical and Cache Results

## Lexical correctness (differential)

`TestSearchDifferentialRandomized` runs 10,000 randomized queries (1/2/3-rune
terms, Unicode, punctuation, FTS operators, empty/whitespace, pagination, limits,
kind filters, cross-project, superseded) over a randomized dataset and requires
byte-identical results to the original scan (`listMemoriesScan`). **PASS.**

`TestSearchMatchesScanAcrossCases` (23 cases) and `TestSearchDifferentialEdgeCases`
also PASS.

## Cache consistency

| Test | Result |
|---|---|
| `TestSearchCacheInvalidatedByWrite` | PASS |
| `TestSearchCacheInvalidatedBySupersede` | PASS |
| `TestSearchCrossStoreVisibility` | PASS (cross-store write visible next query) |
| `TestSearchIndexFallsBackWhenIndexTableDropped` | PASS |
| `TestSearchRecoversAfterIndexDrop` | PASS (write survives; index rebuilt) |
| `TestSearchIndexRebuildIsIdempotent` | PASS |

Generation is read before computing a result, so a concurrent write cannot cause a
stale result to be stored under the new generation (fixed in V3; regression test
present).

## Latency

Small sizes (Phase 1, 100–10K) are dominated by fixed store-open cost (~0.2–1 ms).
The 1M lexical scaling numbers (pod, V6) are in `V6_CONSOLIDATED_RESULTS.md`;
they are not re-run here (no 1M in Phase 1).

## Pass condition

Identical supported query semantics and no stale/unauthorized results in the
defined tests: **PASS**.
