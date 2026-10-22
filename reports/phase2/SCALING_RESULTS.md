# Phase 2 — Scaling Results

## Summary

The engine was exercised at 1,000 / 10,000 / 25,000 / 50,000 / 100,000
memories. The **honest maximum validated size is 100,000 memories** on this
hardware. 1M was explicitly out of scope (memory) and was not run. Resource use
is bounded and grows roughly linearly with the corpus.

## Environment

Apple M5, 10 logical CPUs, 16 GB unified memory, macOS 26.6.2, go1.27.3
darwin/arm64. Worktree base `85e6ea1c`.

## HNSW construction scaling (optimized, `M=64`, dim=128)

| n | build ms | vectors/s | peak RSS |
|---|---|---|---|
| 1,000 | 192 | 5,186 | 21 MB |
| 10,000 | 4,745 | 2,107 | 55 MB |
| 25,000 | 14,226 | 1,757 | 114 MB |
| 50,000 | 30,236 | 1,654 | 195 MB |
| 100,000 | 68,429 | 1,461 | 387 MB |

Build time grows slightly super-linearly (the `log n` graph depth), memory
linearly. Recall@10 at 100K = 0.9949.

**Extrapolation to 1M:** ~10–12 minutes single-threaded and ~3.5–4 GB peak for
the graph (128 dims, `M=64`), plus the loaded vectors. It fits in 16 GB but was
not run — the Phase 2 brief forbids 1M HNSW and the required matrix tops out at
100K. This is an estimate, not a measurement.

## Retrieval scaling

Lexical and semantic retrieval were already measured in Phase 1 (1M lexical
old-only 3,387 ms → 1.60 ms; no-match 3,263 ms → 0.61 ms; ANN 100K 59.2 ms exact
→ 1.20 ms). This phase did not re-measure 1M lexical (out of scope) and does not
claim those numbers as its own.

## Concurrency scaling (see `CONCURRENCY_SOAK.md`)

Short 10 s runs at 1/4/8/16/32 concurrent clients produced **zero errors and zero
SQLITE_BUSY** at every level against a disposable database, with read p99 from
0.47 ms (1 client) to 15.2 ms (32 clients) and write p99 from 1.9 ms to 1.34 s.
The single-writer SQLite model is the scaling limit: write tail latency grows
sharply above ~8 concurrent writers, absorbed by `busy_timeout=5000`.

## Resource bounds

- Search result cache: bounded at 32 MB with LRU eviction.
- Derived-index rebuild: batched (`searchRebuildBatch=5000`) so a large store is
  not held in memory at once.
- Embedding rebuild: batched (1000 rows/transaction).
- Concurrency stress: no goroutine leak (before 3 / after 2), heap stable at a
  few MB after GC across all levels.

## Maximum validated

| Dimension | Validated |
|---|---|
| Memories (build + recall) | **100,000** |
| Concurrent clients | 32 (correct; write tail degrades) |
| Concurrent writers before tail degradation | ~8 |
| Neural embeddings at scale | NOT TESTED here (Codex's Mac/neural workstream) |
