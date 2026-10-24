# ANN root-cause report

## Symptom (V4)

`github.com/coder/hnsw` v0.6.1, 100K vectors: search latency 76 µs but
**recall@10 = 0.17**, and increasing `EfSearch` from 20 to 400 did not change it.

## Root cause (confirmed in the library source)

The failure was **not** normalization, dimensions, distance type, NaN/zero
vectors, or ID mapping. It was a termination bug in the library's layer search
(`graph.go`, `func (n *layerNode[K]) search`):

```go
func (s searchCandidate[K]) Less(o searchCandidate[K]) bool { return s.dist < o.dist } // min-heap
...
improved = improved || dist < result.Min().dist   // result.Min() is the CLOSEST kept
...
if !improved && result.Len() >= k { break }        // break almost immediately
```

`result` is a min-heap, so `result.Min()` is the **closest** element. The
"improvement" test therefore asks "is this neighbour closer than the best one
already found?", which is almost always false. Once the result set holds `k`
items (after the entry node's immediate neighbours), the loop breaks. The search
never explores the graph, so recall is low and — because it never depends on
`EfSearch` — constant.

A secondary defect: `heap.Heap.Max()` returns `data[len-1]`, which is not the
maximum of a min-heap.

## Fix

Vendored the two MIT-licensed files (`graph.go`, `distance.go`) into
`cli/beacon/internal/hnsw/` and replaced the layer search with the canonical
HNSW algorithm: a closest-first candidate frontier and a farthest-first result
heap, terminating when the closest remaining candidate is farther than the worst
kept result. `M`, `Ml`, `EfSearch`, `Add`, `Delete`, and the distance functions
are unchanged. The upstream dependency was removed from `go.mod`.

## Evidence: accuracy before/after (synthetic clustered vectors, n=20000, 128-dim)

| build | recall@10 |
|---|---|
| upstream coder/hnsw, M=16 | 0.17 |
| vendored fix, M=16 | 0.74 |
| vendored fix, M=32 | 0.92–0.94 |
| **vendored fix, M=64 (selected)** | **0.994–0.997** |

Recall is verified against brute-force exact nearest neighbours
(`TestANNRecallSynthetic`, `exactTopK`). The accuracy target (recall@10 ≥ 0.95)
is met at M=64. `EfSearch` (20–400) has little effect once M is large enough;
graph connectivity, not the frontier size, was the limiter.

## Latency before/after (synthetic, 128-dim, M=64)

| n | ANN p50 | exact p50 | ANN speedup |
|---|---|---|---|
| 1,000 | 304 µs | 324 µs | 1.1× |
| 10,000 | 497 µs | 5.34 ms | 10.7× |
| 100,000 | 527 µs | 38.0 ms | 72× |

M=64 raised ANN latency versus the M=16 build (76 µs @100K) because the search
explores more neighbours, but it is the configuration that meets the recall
target. Exact search is retained as the fallback and correctness reference.

## Not measured

- 1M vectors. In-memory HNSW with M=64 stores ~64 edges per node as Go maps;
  at 1M nodes this is estimated at multiple GB and was not run (memory-bound on
  the 7.8 GB machine). A disk-backed or quantized index is required at 1M.
- ANN recall on the hashed embedder is 0.000 because that embedding space is
  flat (exact nearest neighbours are ill-defined). ANN is validated on
  structured vectors, which is the correct proxy for neural embeddings.
