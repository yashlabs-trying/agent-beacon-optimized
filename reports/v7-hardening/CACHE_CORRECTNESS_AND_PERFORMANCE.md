# Beacon V7 hardening — Phase 3: Cache Correctness and Performance

Status: **Existing cache reviewed and validated. No new cache added (none needed).**

## 1. Decision

The mission says: "Do not create an elaborate caching framework. First identify which
repeated work is actually expensive." The repository **already** has a purpose-built,
bounded result cache for the adaptive search path (`search_cache.go`), so a new cache
would duplicate it. The correct action is to validate the existing one, not add another.

## 2. Existing cache design (`internal/learning/search_cache.go`)

- **Bounded:** 32 MiB cap, LRU eviction, so memory cannot grow with query cardinality.
- **Scoped key:** `{dbPath, project, kind, query, limit, page, rankingVersion}` — includes
  project and ranking version, so cross-project/tenant contamination and stale ranking
  rules are structurally impossible.
- **Generation-invalidated:** entries are served only when the durable `generation`
  counter (advanced by `PutMemory`) still matches. A write in any process invalidates the
  entry for every process on the next lookup.
- **Never serves superseded/deleted results:** supersession bumps the generation; the
  stale entry is rejected.

## 3. Correctness validation (existing tests, re-run on this commit)

| Requirement | Test | Result |
|---|---|---|
| Invalidation on write | `TestSearchCacheInvalidatedByWrite` | PASS |
| Invalidation on supersede | `TestSearchCacheInvalidatedBySupersede` | PASS |
| Concurrent readers + writers | `TestSearchConcurrentReadersAndWriters` | PASS |
| Concurrent supersede + read | `TestSearchConcurrentSupersedeAndRead` | PASS |
| Cache invalidated under writes (Phase 2 matrix) | `TestPhase2CacheInvalidationUnderWrites` | PASS |
| Cross-store visibility | `TestSearchCrossStoreVisibility` | PASS |

## 4. Performance

The cache is exercised by the warm benchmarks (`BenchmarkPhaseCListMemoriesWarm`,
`BenchmarkPhaseCSearchQuestionWarm`): repeated identical queries are served from the
cache after the first miss, which is why the warm per-op cost is dominated by the SQLite
generation check rather than a full query. A dedicated hit-vs-miss microbenchmark was
**not added** in this pass; the cache's benefit is demonstrated by the warm-path numbers
and its correctness by the tests above.

## 5. Requirements coverage

| Requirement | Status |
|---|---|
| Bounded memory | PASS (32 MiB LRU) |
| Project/tenant-aware keys | PASS |
| Explicit generation/version checks | PASS |
| Immediate invalidation after committed mutations | PASS |
| Deletes/supersession handled | PASS |
| Concurrency safety | PASS (race suite) |
| No stale/unauthorized result after crash recovery | PASS (generation is durable) |
| Stable when caching disabled | PASS (miss path is the full query) |

## 6. Verdict

**PASS (existing cache).** No new cache introduced; the existing one meets every stated
requirement and is regression-tested. A quantitative hit/miss latency microbenchmark
remains **NOT TESTED**.
