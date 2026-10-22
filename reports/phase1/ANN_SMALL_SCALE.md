# Phase 1 — ANN Small-Scale Correctness (22 October 2026 Mac revalidation)

On the Apple M5 Mac, `BEACON_ANN_MATRIX=1 go test -run '^TestANNPhase1Matrix$' -v -timeout 30m ./internal/learning` tested 128-dimensional synthetic clustered vectors (seed 7), 64 queries per cell, exact brute-force top-10 ground truth, n=100/1K/5K/10K and M=16/32/64. Raw 12-cell results with recall@1, p50/p95/p99, heap, and allocations are in `artifacts/ann_matrix.txt`.

| n | M16 R@10 / build s | M32 R@10 / build s | M64 R@10 / build s |
|---:|---:|---:|---:|
| 100 | 0.9547 / 0.012 | 0.9984 / 0.021 | 1.0000 / 0.030 |
| 1,000 | 0.8703 / 0.126 | 0.9625 / 0.338 | 0.9953 / 0.947 |
| 5,000 | 0.7516 / 0.785 | 0.9844 / 2.436 | 1.0000 / 7.646 |
| 10,000 | 0.8187 / 1.731 | 0.9844 / 5.240 | 1.0000 / 17.177 |

At 10K, M64 search p50/p95/p99 was 132/149/155 microseconds, versus 40/47/51 microseconds at M16. M64 reported 65 MB post-build Go heap, about 5.07 GB cumulative build allocations, and 270 allocations per search. The latter two are optimization targets; post-build heap is not process peak RSS. A repeated 12-cell CPU-profile run (`BEACON_ANN_MATRIX=1 go test -run '^TestANNPhase1Matrix$' -cpuprofile=ann_cpu.pprof ./internal/learning`, 37.78 s) attributed 82.31% cumulative samples to `Graph.Add`, with 54.25% in layer search and 28.09% in neighbor addition; cosine similarity alone took 31.08% flat samples. The profile and top output are `artifacts/ann_cpu.pprof` and `artifacts/ann_profile_top.txt`. M64 reaches the small-scale recall target, but build cost remains a blocker for larger scale. No 1M ANN run was attempted.

`TestANNInsertionOrderAndDuplicates`, `TestANNRejectsInvalidVectors`, and `TestHybridANNPath` passed. A dimension mismatch previously caused a panic during graph operations; the regression now rejects zero, NaN, infinite, empty, and mismatched vectors, and skips stored embeddings whose dimension disagrees with the embedder. The matrix is synthetic and seed-specific; it does not establish recall on real neural embeddings. Earlier WSL and paid-pod ANN results belong to the prior record below.

## Prior Phase 1 report (preserved below)

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
