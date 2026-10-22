# Phase 2 — Concurrency, Stress and Soak

## Summary

Against disposable databases, the store is **race-free** (`go test -race
./internal/learning/` clean, 417 s) and **error-free at every tested concurrency
level in normal builds** (1/4/8/16/32 clients, 0 errors, 0 SQLITE_BUSY). A
15-minute soak at 8 clients passed with 69,093 reads / 18,348 writes / 4,318
supersessions and zero errors. The single-writer SQLite model is the scaling
limit: write tail latency grows sharply above ~8 concurrent writers. Frequent
full derived-index rebuilds cause reader tail-latency spikes.

## Environment

Apple M5, 10 logical CPUs, 16 GB, macOS 26.6.2, go1.27.3 darwin/arm64. Worktree
base `85e6ea1c`. Each run uses a fresh `t.TempDir()` database.

## Harness

`TestPhase2ConcurrencyStress` (env-driven): mixed workload — 20% writes, ~6%
supersession-under-read, rest reads (lexical + hybrid); a churn goroutine drops
the derived index every 2 s to force rebuilds during reads and writes. Records
throughput, error/busy counts, latency percentiles, goroutine count, heap and WAL.

## Matrix — 10 s per level, normal build

| clients | reads | writes | supersedes | errors | busy | read p50/p95/p99 (µs) | write p50/p95/p99 (µs) |
|---|---|---|---|---|---|---|---|
| 1 | 5,114 | 1,358 | 320 | 0 | 0 | 343 / 398 / 473 | 1,142 / 1,529 / 1,855 |
| 4 | 7,797 | 2,069 | 487 | 0 | 0 | 292 / 568 / 716 | 1,375 / 4,168 / 18,996 |
| 8 | 6,406 | 1,698 | 402 | 0 | 0 | 597 / 1,240 / 1,648 | 2,916 / 20,643 / 57,414 |
| 16 | 6,280 | 1,657 | 391 | 0 | 0 | 896 / 1,898 / 3,331 | 5,567 / 106,364 / 338,992 |
| 32 | 5,964 | 1,574 | 371 | 0 | 0 | 1,055 / 2,557 / 15,183 | 10,268 / 534,546 / 1,336,097 |

**Findings:** reads stay sub-4 ms at p99 up to 16 clients; at 32 clients read p99
jumps to 15 ms. Write p99 grows from 1.9 ms (1 writer) to 1.34 s (32 writers) —
SQLite serialises writers (`_txlock=immediate`), and `busy_timeout=5000` absorbs
the wait with zero errors. **~8 concurrent writers is the practical knee.**

## Soak — 15 minutes, 8 clients

| metric | value |
|---|---|
| reads | 69,093 |
| writes | 18,348 |
| supersessions | 4,318 |
| errors / busy | **0 / 0** |
| read latency p50 / p95 / p99 / mean | 712 µs / 1,486 µs / **654,958 µs** / 19,676 µs |
| write latency p50 / p95 / p99 / mean | 8,758 µs / 116,025 µs / **748,680 µs** / 38,754 µs |
| goroutines before / after | 3 / 2 (no leak) |
| heap alloc / sys | 5.3 MB / 35.1 MB |
| GCs | 317,093 |
| result | **PASS** |

**Interpretation.** The p50/p95 are healthy (read 0.7/1.5 ms), but p99 is ~0.65 s
for reads and ~0.75 s for writes, and the mean read latency (19.7 ms) is far above
the median. This is caused by the harness's deliberate churn: every 2 s the
derived FTS index is marked not-ready, and the next search performs a **full
index rebuild inside a write transaction**, which holds the write lock and blocks
readers (immediate transactions) until it completes. The corpus grows over the
15 minutes (18 k writes on top of the seed), so rebuilds get slower and the tail
grows. In production a rebuild only happens after the index is detected unhealthy,
not every 2 s, so this is a worst-case tail, not the steady-state.

## Race detection

- `go test -race ./internal/learning/ -count=1` (full package, all Phase 2 tests):
  **PASS, 417 s, no data races.**
- `-race` stress at 16 clients: no data race. Under race instrumentation the
  process is ~10× slower and a write can exceed `busy_timeout`, producing an
  occasional SQLITE_BUSY; the harness tolerates busy-only errors under `-race`
  and still fails on any other error. This is a timeout artifact, not a race.

## Other observations

- Each `PutMemory`/`ListMemories` call opens and closes its own `*sql.DB`, so the
  317 k GCs in 900 s reflect many short-lived allocations rather than a leak
  (heap stayed at ~5 MB and goroutines returned to baseline). A pooled connection
  would reduce GC pressure but is out of scope.
- WAL size reported 0 at process exit (checkpointed on close).

## Reproduce

```
BEACON_PHASE2_STRESS=1 BEACON_PHASE2_CONCURRENCY=8 BEACON_PHASE2_SECONDS=900 \
  go test ./internal/learning -run TestPhase2ConcurrencyStress -v -count=1 -timeout 25m
```
