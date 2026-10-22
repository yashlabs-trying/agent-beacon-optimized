# Phase 1 — ANN Small-Scale Correctness

Corrected `internal/hnsw` (vendored). Synthetic clustered vectors, 128-dim, 256
clusters, seed 7. Recall measured against brute-force exact nearest neighbours.

## Recall@10 and build (WSL2, measured)

| n | M | recall@10 | build ms | ANN p50 | exact p50 |
|---|---|---|---|---|---|
| 100 | 64 | **1.0000** | 55 | 0.11 ms | 0.043 ms |
| 1,000 | 64 | **0.9910** | 2,171 | 0.37 ms | 0.36 ms |
| 5,000 | 64 | **1.0000** | 16,390 | 0.94 ms | 1.57 ms |
| 10,000 | 64 | **1.0000** | 39,373 | 0.36 ms | 2.49 ms |
| 100 | 32 | 0.9973 | 20 | 0.15 ms | 0.082 ms |
| 1,000 | 32 | 0.9719 | 878 | 0.23 ms | 0.58 ms |
| 5,000 | 32 | 0.9883 | 4,990 | 0.19 ms | 2.39 ms |
| 10,000 | 32 | 0.9727 | 11,544 | 0.84 ms | 3.98 ms |

M=64 meets the ≥0.95 target at every size. M=32 dips to 0.97. M=16 (measured
separately at 20K) was 0.74 — below target.

## Construction bottleneck (investigated, not "fixed")

Build cost is ~3.9–5 ms/node at M=64, single-threaded (`Graph.Add`). It is the
scaling ceiling (1M M=64 ≈ 83 min; measured on the pod). Profiling confirms the
cost is in the per-insert greedy search over a Go-map adjacency, not in vector
generation. **No speculative parallel construction was implemented** — correctness
of a parallel build was not established. This remains the top Phase 2 item.

## Additional checks

- `TestANNRecallFlatSpace` documents that with the hashed embedder the space is
  flat and recall-vs-exact is meaningless (0.000); ANN is only meaningful on
  structured (neural) vectors.
- ID mapping: `memoriesByIDs` hydrates ANN IDs and filters supersession; covered
  by `TestHybridANNPath` (PASS, no cross-project leak).
- Duplicate/zero/NaN vectors and insertion order: not separately tested in Phase 1
  (**NOT TESTED**).
- Index save/reload: not wired (in-memory only) — **NOT TESTED**.

## Pass condition

Recall@10 ≥ 0.95 on representative workloads at M=64: **PASS**. Configurations
that miss it (M=16, some M=32) are documented, not hidden.
