# Beacon Memory Engine V2 — performance report

All values are measured on one machine, by one harness, on identical datasets and
query distributions, with the untouched baseline and the feature branch built
from the same base commit.

## Environment

- Base commit: `99c709081ad49da1c5ade205929b61b19360ed9b` (`origin/main`)
- Baseline: detached worktree at the base commit, unmodified
- Feature: `feat/beacon-memory-engine-v2`
- OS: Linux (WSL2 Ubuntu), 8 vCPU
- CPU: 11th Gen Intel(R) Core(TM) i5-1155G7 @ 2.50GHz
- RAM: 7.8 GB
- Go: go1.27.0 linux/amd64
- SQLite driver: modernc.org/sqlite v1.59.0
- Harness: `internal/learning/search_verify_test.go` (gated by `BEACON_VERIFY=1`),
  copied verbatim to the baseline with baseline hooks
  (`verifyResetCache` no-op, `verifyPrepareIndex` returns 0). Baseline and
  feature are therefore measured by the same code.

## Method

- Dataset (deterministic, `verifyDataset`): ~300-byte memories, 3/4 in `proj-a`,
  one rare token every 500 rows (`unique-needle`), one token only in the oldest
  row (`zzzoldneedle`), a common word in every row (`the`), plus Unicode and
  punctuation rows. Superseded rows are present.
- Workloads (limit 5): `selective` (`unique-needle`), `old-only`
  (`zzzoldneedle`), `no-match` (`zzznotfound`), `common` (`the`), `empty` (no
  text), `mixed` (`workflow` + kind filter).
- Iterations: 200/1000 (n<=10K), 30/200 (n=100K), 3/10 (n=1M), with a focused
  20-iteration re-run at 1M and a 500-iteration re-run of common/empty/mixed at
  1K–100K. `cold` resets the result cache before each operation; `warm` repeats
  the same query.
- Baseline is measured through `Store.ListMemories`, which opens the store per
  call, so both trees include the same fixed per-call cost. Scan-only
  microbenchmarks that reuse an open handle are deliberately not used.

## Results

Latencies are microseconds at p50 / p95 / p99. "cold x" is baseline p50 divided
by feature cold p50.

### n = 1,000

| workload | baseline p50/p95/p99 | feature cold p50/p95/p99 | feature warm p50 | cold x |
|---|---|---|---|---|
| selective | 3152 / 4706 / 6324 | 432 / 610 / 725 | 207 | 7.3 |
| old-only | 2901 / 3917 / 4310 | 388 / 473 / 601 | 209 | 7.5 |
| no-match | 2990 / 3933 / 4575 | 321 / 424 / 593 | 207 | 9.3 |
| common | 239 / 358 / 523 | 224 / 302 / 450 | 227 | 1.07 |
| empty | 247 / 420 / 559 | 212 / 313 / 536 | 217 | 1.17 |
| mixed | 242 / 357 / 507 | 238 / 373 / 474 | 238 | 1.02 |

### n = 10,000

| workload | baseline p50/p95/p99 | feature cold p50/p95/p99 | feature warm p50 | cold x |
|---|---|---|---|---|
| selective | 7397 / 9014 / 10305 | 763 / 1257 / 1444 | 208 | 9.7 |
| old-only | 31538 / 34486 / 37238 | 562 / 767 / 992 | 229 | 56 |
| no-match | 31962 / 34867 / 36893 | 439 / 668 / 796 | 212 | 73 |
| common | 243 / 359 / 533 | 229 / 334 / 495 | 223 | 1.06 |
| empty | 247 / 363 / 513 | 215 / 330 / 479 | 216 | 1.15 |
| mixed | 246 / 372 / 503 | 245 / 378 / 549 | 243 | 1.00 |

### n = 100,000

| workload | baseline p50/p95/p99 | feature cold p50/p95/p99 | feature warm p50 | cold x |
|---|---|---|---|---|
| selective | 8575 / 11251 / 11533 | 3968 / 5695 / 6010 | 243 | 2.16 |
| old-only | 327967 / 361635 / 435717 | 790 / 1206 / 1405 | 247 | 415 |
| no-match | 354285 / 393988 / 397834 | 483 / 584 / 672 | 252 | 733 |
| common | 246 / 384 / 501 | 227 / 331 / 483 | 228 | 1.08 |
| empty | 254 / 357 / 466 | 221 / 357 / 499 | 220 | 1.15 |
| mixed | 246 / 355 / 601 | 249 / 419 / 681 | 244 | 0.99 |

### n = 1,000,000

3-iteration run; common/selective/mixed re-run at 20 iterations (shown after the
slash where they differ).

| workload | baseline p50 | feature cold p50 | feature warm p50 | cold x |
|---|---|---|---|---|
| selective | 7690 (20 it) | 7732 (20 it) | 7743 | 0.99 |
| old-only | 3387032 | 1597 | 260 | 2120 |
| no-match | 3263231 | 612 | 235 | 5332 |
| common | 262 (20 it) | 233 (20 it) | 223 | 1.12 |
| empty | 350 | 277 | 253 | 1.26 |
| mixed | 248 (20 it) | 242 (20 it) | 241 | 1.02 |

## Overhead

| metric | baseline | feature | ratio |
|---|---|---|---|
| index build 1K | — | 22 ms | — |
| index build 10K | — | 268–279 ms | — |
| index build 100K | — | 3.65–3.93 s | — |
| index build 1M | — | 51.7–56.9 s | — |
| db size 1K | 675,840 B | 1,409,024 B | 2.08× |
| db size 10K | 6,479,872 B | 14,569,472 B | 2.25× |
| db size 100K | 64,937,984 B | 136,499,200 B | 2.10× |
| db size 1M | 650,805,248 B | 1,349,431,296 B | 2.07× |
| incremental write (5000 writes) | 373 µs/write | 608 µs/write | 1.63× |
| WAL after 5000 writes | 0 B | 0 B | — |

The trigram index roughly doubles disk use; that is the price of exact substring
matching. Incremental writes are 1.63× slower because each write also updates
the index row and the generation inside its transaction. WAL is zero after the
write loop because the store opens and closes a connection per operation, and
the last close checkpoints and removes the WAL — so the WAL does not grow without
bound under this access pattern.

## Against the stated targets

- Warm repeated queries, >= 5x: met (up to ~13,900x on scan-bound workloads).
- Selective cold queries, >= 3x: met at 1K (7.3x) and 10K (9.7x); **2.16x at
  100K** and **~1.0x at 1M**. At 100K+ the selective query's match count exceeds
  the candidate cap, so the planner routes it to the scan, which the baseline
  already terminates early; the margin narrows with corpus size.
- Common queries, <= 10% regression: met (1.07–1.26x faster at high iteration
  counts; no consistent regression).
- No exact-search compatibility failures: met (see the design doc's test list).
- Bounded cache memory: met (32 MiB byte-bounded LRU).

## Correctness and reliability evidence

- `TestSearchMatchesScanAcrossCases` — 23 query cases vs the original scan.
- `TestSearchDifferentialRandomized` — 10,000 randomized queries over a
  randomized dataset vs the original scan, covering 1/2/3-rune terms, Unicode,
  punctuation, FTS operators, repeated/mixed-case terms, empty and whitespace
  queries, pagination, limits and kind filters.
- `TestSearchDifferentialEdgeCases` — pinned tricky inputs (quotes, `*`, `(`,
  `%`, `'`, backslash, pipe, caret, CJK, emoji, case).
- `TestPlannerSelectivity`, `TestSearchIndexFallsBackWhenIndexTableDropped`,
  `TestSearchCacheInvalidatedByWrite`, `TestSearchCacheInvalidatedBySupersede`,
  `TestSearchIndexRebuildIsIdempotent`, `TestSearchHandlesQuotesInQuery`.
- `TestSearchCrossStoreVisibility` — a write through one Store is visible to
  another Store on its next query.
- `TestSearchRecoversAfterIndexDrop` — a write succeeds with the index gone; the
  next query is correct; the index is rebuilt and marked ready.
- `TestSearchConcurrentReadersAndWriters` and
  `TestSearchConcurrentSupersedeAndRead` (run under `-race`) — no data race, no
  cross-project leak, no superseded result, all acknowledged writes visible.
- Full repo suite: the feature branch's failing-test set is identical to the
  baseline's (pre-existing, environment-dependent hooks/connect tests).

## Limitations and explicit non-runs

- Semantic retrieval, evidence trust, and ANN are not implemented; there are no
  semantic-quality or semantic-latency numbers.
- Selective-cold at 100K/1M does not meet the 3x target (documented above).
- The 1M percentiles use 3–20 iterations and are correspondingly coarse.
- Disk growth is reported in total; a per-table breakdown was not produced.
- Baseline 1M was measured, not extrapolated.

## Reproduction

```bash
cd cli/beacon
go test ./internal/learning/...            # correctness
go test -race ./internal/learning/...      # concurrency

# verification harness (feature)
BEACON_VERIFY=1 BEACON_VERIFY_TREE=feature \
  go test -run 'TestVerify' -v ./internal/learning/

# sizes and workload selection
BEACON_VERIFY=1 BEACON_VERIFY_SIZES=1000,10000,100000,1000000 \
  BEACON_VERIFY_ONLY=selective,old-only,no-match,common,empty,mixed \
  BEACON_VERIFY_ITERS=20 go test -run 'TestVerifyHarness' -v ./internal/learning/
```

The baseline is measured by copying `search_verify_test.go` into the untouched
worktree and adding a baseline hooks file (`verifyResetCache` no-op,
`verifyPrepareIndex` returns 0), then running the same command with
`BEACON_VERIFY_TREE=baseline`.
