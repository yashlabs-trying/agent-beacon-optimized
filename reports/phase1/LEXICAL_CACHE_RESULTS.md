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
