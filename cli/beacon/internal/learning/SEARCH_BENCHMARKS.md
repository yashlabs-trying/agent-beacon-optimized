# Beacon Memory Engine V2 — performance report

All numbers below were produced on the same machine and dataset generator, with
the untouched baseline and the feature branch from the same base commit.

## Environment

- Base commit: `99c709081ad49da1c5ade205929b61b19360ed9b` (`origin/main`)
- Baseline: detached worktree at the base commit, unmodified
- Feature: `feat/beacon-memory-engine-v2`
- OS: Linux (WSL2 Ubuntu), 8 vCPU
- CPU: 11th Gen Intel(R) Core(TM) i5-1155G7 @ 2.50GHz
- RAM: 7.8 GB
- Go: go1.27.0 linux/amd64
- SQLite driver: modernc.org/sqlite v1.59.0
- Dataset: deterministic, seed-free generator in `search_test.go` / the baseline
  harness. ~300-byte memories, 3/4 in `proj-a`, one rare token every 500 rows,
  one token only in the oldest row (`zzzoldneedle`), a common word in every row
  (`the`), plus Unicode and punctuation rows.
- Workloads: `unique-needle` (selective, recent matches), `zzzoldneedle` (match
  only in the oldest row), `zzznotfound` (no match), `the` (common), limit 5.

Baseline is measured through the public `Store.ListMemories` entry point, which
re-opens the store per call, so it includes the same per-call setup the feature
path does. Scan-only microbenchmarks that reuse an open handle are not used for
the comparison because they understate the baseline's fixed cost.

## Results (ns/op; lower is better)

n = 1,000

| workload | baseline | V2 cold | V2 warm | cold speedup | warm speedup |
|---|---|---|---|---|---|
| selective | 5.16 ms | 0.639 ms | 0.284 ms | 8.1x | 18x |
| common | 0.549 ms | 0.369 ms | — | 1.49x faster | — |
| old-only | 5.13 ms | 0.609 ms | 0.276 ms | 8.4x | 18.6x |
| no-match | 5.11 ms | 0.494 ms | — | 10.3x | — |

n = 10,000

| workload | baseline | V2 cold | V2 warm | cold speedup | warm speedup |
|---|---|---|---|---|---|
| selective | 12.8 ms | 1.24 ms | 0.298 ms | 10.3x | 43x |
| common | 0.583 ms | 0.371 ms | — | 1.57x faster | — |
| old-only | 45.1 ms | 0.836 ms | 0.263 ms | 54x | 171x |
| no-match | 41.7 ms | 0.559 ms | — | 75x | — |

n = 100,000

| workload | baseline | V2 cold | V2 warm | cold speedup | warm speedup |
|---|---|---|---|---|---|
| selective | 13.9 ms | 5.67 ms | 0.283 ms | 2.45x | 49x |
| common | 0.578 ms | 0.389 ms | — | 1.49x faster | — |
| old-only | 420 ms | 1.11 ms | 0.270 ms | 378x | 1555x |
| no-match | 383 ms | 0.674 ms | — | 568x | — |

n = 1,000,000 (feature only; baseline old-only/no-match extrapolate to ~4–5 s)

| workload | V2 cold |
|---|---|
| selective | 10.9 ms |
| old-only | 1.42 ms |
| no-match | 0.61 ms |
| common | 0.40 ms |

One-time index build: 4.5 s at 100K, 61.7 s at 1M. Database size at 1M: 1.33 GB
(the trigram index roughly doubles the file). Index rows = memory rows.

## Against the stated targets

- Warm repeated queries, >= 5x: met (18x–1555x).
- Selective cold queries, >= 3x: met at 1K (8.1x) and 10K (10.3x); **not met at
  100K (2.45x)**. At 100K the selective cold path is dominated by the trigram
  `MATCH` over a large corpus and the fixed store-open cost; the scan it beats
  early-terminates, so the margin narrows.
- Common queries, <= 10% regression: met (no regression; 1.49–1.57x faster).
- No exact-search compatibility failures: met (see below).
- Bounded cache memory: met (32 MiB byte-bounded LRU).

## Correctness and reliability evidence

- `TestSearchMatchesScanAcrossCases`: 23 query cases (case, Unicode, punctuation,
  multi-term, empty, 1- and 2-character, cross-project, kind filter, pagination)
  compared against the original scan.
- `TestPlannerSelectivity`: a common term overflows the cap and routes to the
  scan; a rare term is served by the index.
- `TestSearchIndexFallsBackWhenIndexTableDropped`: results are unchanged after the
  index table is dropped.
- `TestSearchCacheInvalidatedByWrite` / `...BySupersede`: a write or supersede is
  visible on the next query.
- `TestSearchIndexRebuildIsIdempotent`: repeated rebuilds leave one index row per
  memory.
- `TestSearchHandlesQuotesInQuery`: FTS operators in user text are inert.
- `TestSearchConcurrentReadersAndWriters` (run under `-race`): 8 readers and 4
  writers; no data race, no cross-project leak, all acknowledged writes visible.
- Full repo suite: the feature branch's failing-test set is identical to the
  baseline's (pre-existing, environment-dependent hooks/connect tests). No new
  failures.

## Not measured / limitations

- Baseline at 1M was not run; old-only/no-match baseline at 1M is extrapolated
  from the 100K figure and clearly labeled as such.
- Disk amplification is reported (1.33 GB at 1M) but not broken down per table.
- No semantic/embedding retrieval is implemented, so no semantic latency or
  quality numbers exist.
- Index build is O(corpus) and one-time; for a very large existing store the
  first search after upgrade pays it. A future change could build incrementally
  or in the background.

## Reproduction

```bash
# toolchain
# Go 1.27 required by go.mod

cd cli/beacon
go test ./internal/learning/...            # correctness
go test -race ./internal/learning/...      # concurrency

# benchmarks (feature)
go test -run '^$' -bench 'BenchmarkScan|BenchmarkAdaptive|BenchmarkIndexBuild' \
  -benchmem ./internal/learning/

# choose sizes
BEACON_BENCH_SIZES=1000,10000,100000,1000000 \
  go test -run '^$' -bench 'BenchmarkAdaptive' -benchmem ./internal/learning/
```

Baseline benchmarks live in a separate harness (`BenchmarkBaseline*`) that is
not part of the feature branch; it calls `Store.ListMemories` on the unmodified
tree with the same generator.
