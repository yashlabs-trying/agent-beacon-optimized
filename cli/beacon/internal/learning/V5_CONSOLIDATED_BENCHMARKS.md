# V5 consolidated benchmarks

Machine: WSL2 Ubuntu, 8 vCPU, 11th Gen Intel(R) Core(TM) i5-1155G7 @ 2.50GHz,
7.8 GB RAM, Go 1.27.0, modernc.org/sqlite v1.59.0. Baseline `99c70908`; V4
`cb9a52d3`; V5 from V4. All values measured unless marked otherwise.

## Executive comparison

| Metric | Original Beacon | V4 | V5 | Improvement |
|---|---|---|---|---|
| Recall@1 (17-case lexical/semantic set) | 0.118 | 0.412 | 0.412 (neural not integrated) | +249% vs original; unchanged from V4 |
| Recall@5 (same set) | 0.412 | 0.706 | 0.706 (neural not integrated) | +71% vs original |
| ANN Recall@10 (structured synthetic) | N/A | 0.17 (rejected) | **0.975** | fixed (see ANN_ROOT_CAUSE_REPORT.md) |
| Retrieval p95 (100K old-only) | 361,635 µs | 1,206 µs | 1,206 µs (lexical unchanged) | ~300× |
| Retrieval at 1M | old-only p50 3,387,032 µs | lexical p50 1,597 µs | lexical 1,597 µs; semantic ANN 1M **not measured** (memory-bound) | ~2000× lexical |
| Context tokens (17 queries) | fixed top-5 2,140 | 1,242 | 1,242 | 1.72× fewer |
| Agent task success | **NOT MEASURED** | **NOT MEASURED** | **NOT MEASURED** | not measured |
| End-to-end latency (agent) | **NOT MEASURED** | **NOT MEASURED** | **NOT MEASURED** | not measured |
| Memory usage | not measured precisely | not measured precisely | ANN M=64 in-memory, **1M not feasible** | see below |
| Disk overhead (100K) | 64.9 MB | 136.5 MB | 136.5 MB | 2.1× tradeoff |

## ANN (V5, synthetic 128-dim, M=64, recall verified vs exact)

| n | ANN p50 | exact p50 | speedup | recall@10 |
|---|---|---|---|---|
| 1,000 | 304 µs | 324 µs | 1.1× | — |
| 10,000 | 497 µs | 5.34 ms | 10.7× | — |
| 100,000 | 527 µs | 38.0 ms | 72× | — |
| 20,000 (recall run) | — | — | — | **0.975** |

## Lexical engine (unchanged from V2)

100K p50: selective cold 3.97 ms, old-only cold 0.79 ms, no-match cold 0.48 ms,
warm 0.24 ms. 1M: old-only cold 1.60 ms, no-match cold 0.61 ms. Full tables in
`SEARCH_BENCHMARKS.md`.

## Semantic exact (V3/V4, unchanged)

1K 5.49 ms, 10K 65.8 ms, 100K 513 ms — linear; small-store only.

## Reliability

Cache correctness, generation safety, fallback, and cross-scope isolation are
verified under `-race` (647 s run) and by fault tests. See RELIABILITY_REPORT.md.

## What V5 changed

1. Fixed the ANN recall bug (0.17 → 0.975) and selected M=64.
2. Wired the corrected ANN into hybrid search as an opt-in path (`UseANN`).
3. Removed the upstream `coder/hnsw` dependency (vendored + fixed).

V5 did **not** change retrieval quality (no neural model) and did **not** produce
agent results.
