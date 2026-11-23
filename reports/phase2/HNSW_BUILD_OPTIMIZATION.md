# Phase 2 — HNSW Build Optimization

## Summary

HNSW index construction was profiled and rewritten. On a matched benchmark the
optimized builder is **3.5–5.6× faster** than the Phase 1 builder at equal `M`
and **~2.8× lower peak RSS**, with **no recall loss** (Recall@10 = 0.9949 at
100K, target ≥ 0.95). A lower-`M` sweep is also reported: `M=32` builds ~2.8×
faster than `M=64` but recall falls to 0.966 at 25K, so `M=64` remains the safe
default.

## Environment

| | |
|---|---|
| Hardware | Apple M5, 10 logical CPUs, 16 GB unified memory |
| OS | macOS 26.6.2 (build 25G83), arm64 |
| Go | go1.27.3 darwin/arm64 |
| Worktree | `/Users/<user>/beacon-phase2-opencode` |
| Branch | `test/beacon-v6-phase2-opencode` |
| Base SHA | `85e6ea1c` (Phase 1 verified V6) |
| HNSW code | `cli/beacon/internal/hnsw/graph.go` (vendored `coder/hnsw` v0.6.1, MIT) |

> These are macOS/Apple-Silicon numbers only. No WSL2/Linux HNSW run was made in
> this session, so no Linux claim is made here. Phase 1's 506 s 100K `M=64` figure
> was on different (slower) hardware and is not directly comparable.

## Method

- Vectors: `syntheticVectors(n, dim=128, clusters=256, seed=7)` — the same
  generator the production recall benchmark uses, so build and recall are
  measured on identical data.
- Matched comparison: the original `graph.go` (from `85e6ea1c`) and the optimized
  `graph.go` were both built and run through the same harness
  (`TestHNSWBuildTiming`, `BEACON_PHASE2_TIMING=1`) on the same machine, one
  after the other, with the same vector set. `M=64`, `Ml=0.25`, `EfSearch=100`.
- Profiling: `go test -cpuprofile -memprofile` on a single `n=10000` build.
- Recall: `BenchmarkANNvsExact` (ground truth by exact cosine top-k), `M=64`.
- Command: `BEACON_PHASE2_TIMING=1 BEACON_PHASE2_ANN_SIZES=... go test
  ./internal/learning/ -run TestHNSWBuildTiming -v -count=1`

## Profiling findings (original builder, n=10000)

| Hotspot | Share | Cause |
|---|---|---|
| `maps.Keys` | **51% of all bytes allocated** (2.65 GB) | neighbour keys copied to a new slice on every frontier expansion |
| `slices.Sort` of neighbour keys | **~32% of search CPU** | re-sorted on every expansion |
| `container/heap` interface boxing | 14.2 M allocations | `Push`/`Pop` box `searchCandidate` into `any` |
| distance recomputation | ~25% of CPU | `addNeighbor` recomputes all edge distances to find the worst |
| neighbour map lookups | ~16% of search CPU | `mapaccess1_faststr` for visited/neighbour lookups |

## Changes

1. **Slice adjacency with cached edge distances.** Each node keeps
   `neighbors []*layerNode` plus `dists []float32` (distance to each neighbour,
   computed once on edge insertion). Iteration is insertion-ordered and
   allocation-free; the worst-neighbour prune is a comparison scan, not a
   recomputation. `M` is small, so linear scans beat map hashing.
2. **Typed heaps.** `container/heap` replaced with typed `push/pop/remove/up/down`
   so no value is boxed into an interface.
3. **Reusable search scratch.** One epoch-stamped `map[*layerNode]int32` visited
   set and two typed heaps are reused per search (a serial scratch for `Add`, a
   `sync.Pool` for concurrent `Search`), so a search allocates nothing after warmup.
4. **Preallocated adjacency** to capacity `M`, eliminating growth reallocation.
5. **`slices.SortFunc`** instead of reflective `sort.Slice` for the final result
   ordering.
6. **Optional decoupled `EfConstruction`** (build beam width) and `Rng` seeding
   for reproducible benchmarks. Both default to the original behaviour.

Algorithm, pruning rule, parameters and public API are unchanged.

## Results — build throughput and memory (matched, `M=64`)

| n | baseline ms | optimized ms | speedup | baseline peak RSS | optimized peak RSS |
|---|---|---|---|---|---|
| 1,000 | 1,072 | 192 | **5.58×** | 31 MB | 21 MB |
| 10,000 | 18,645 | 4,745 | **3.93×** | 124 MB | 55 MB |
| 25,000 | 49,775 | 14,226 | **3.50×** | 281 MB | 114 MB |
| 50,000 | 109,680 | 30,236 | **3.63×** | 542 MB | 195 MB |
| 100,000 | ~250,000 (extrapolated) | **68,429** | ~3.6× | — | 387 MB |

Vectors/second at 100K: **1,461/s**. Allocations at n=10000: **5.4 GB / 18.3 M
allocs → 40.7 MB / 175 K allocs** (measured, `-benchtime=1x`).

## Results — Recall@10 (optimized, `M=64`, clusters=256, seed=7)

| n | Recall@10 | Exact-truth ms |
|---|---|---|
| 10,000 | **1.0000** | 406 |
| 25,000 | **1.0000** | 1,096 |
| 100,000 | **0.9949** | 5,135 |

Recall is within noise of the Phase 1 reported 0.9953 at 100K, so the speedup
does not cost recall.

## `M` / `EfConstruction` sweep (n=25,000, optimized builder)

| Config | Build ms | Recall@10 |
|---|---|---|
| M=16 | 2,035 | 0.729 |
| M=32 | 5,142 | 0.966 |
| M=64 | 14,226 | 1.000 |
| M=32, EfConstruction=200 | 4,986 | 0.964 |
| M=32, EfConstruction=400 | 5,392 | 0.960 |
| M=32, EfConstruction=800 | 5,610 | 0.983 |

A wider construction beam does not close the low-`M` recall gap; the limiter is
graph degree, not construction exploration. The principled next step for low-`M`
recall is the canonical diversity neighbour-selection heuristic
(Malkov–Yashunin Algorithm 4), which is **not** implemented here. `M=64` stays
the default.

## Determinism

The build is **not bit-reproducible** — this is pre-existing: `newANNIndex` left
`Rng` time-seeded and `layer.entry()` returns an arbitrary map element. The
optimization does not make it worse and does not make it better. `annParams.Seed`
now allows reproducible level generation for benchmarks. Recall is stable across
runs. Reported as a known limitation, not a regression.

## Correctness / regression coverage

`internal/hnsw/graph_test.go`: structural invariants (no duplicate neighbours,
`len ≤ M`, `len(dists)==len(neighbors)`, neighbours present in the layer, cached
distances equal recomputed distances), ordered search output, and exact recall
against brute force on clustered data (1.000). `TestANNRecallSynthetic` (1.000),
`TestHybridANNPath`, and the full learning suite pass; `go test -race
./internal/learning/` is clean.

## Limitations

- 100K baseline was extrapolated, not measured (baseline 100K would take ~4 min;
  the required matrix tops out at 100K and 1M HNSW was explicitly out of scope).
- `runtime.madvise` dominates sampled CPU on macOS (Go scavenger); raising
  `GOGC` or disabling it did not change wall time (5.10 s vs 4.95 s), so it is a
  profiling artifact, not the bottleneck.
- Low-`M` recall is unresolved (diversity heuristic not implemented).
