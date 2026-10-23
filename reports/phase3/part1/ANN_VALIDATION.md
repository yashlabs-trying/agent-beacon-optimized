# Phase 3A — Bounded ANN Validation (Workstream B)

**Status: PASS within the bounded envelope. M=64 retained as the verified reference.
No default changed.**

## 1. Method

- Synthetic clustered unit vectors (`syntheticVectors(n, dim=128, clusters=256)`), the
  same generator used by the Phase 1/2 ANN harnesses; vector seed 7, graph RNG seed 99.
- 64 fixed queries per size; ground truth = brute-force exact cosine top-10 (`exactTopK`).
- Recall@1 and Recall@10 measured against exact neighbours.
- Corrected HNSW implementation (`internal/hnsw`), `M=64`, `EfSearch=100`, `Ml=0.25`.
- Determinism checked by rebuilding with the same seed and requiring identical results.
- `peakRSSKB` sampled before/after each build.

## 2. Results (M=64)

| Vectors | Recall@1 | Recall@10 | Build ms | p50 µs | p95 µs | search allocs | RSS before→after |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 1,000 | 1.0000 | 0.9953 | 151 | 28 | 44 | 7.0 | 12→16 MB |
| 10,000 | 1.0000 | 1.0000 | 4,483 | 51 | 106 | 8.0 | 35→47 MB |
| 25,000 | 1.0000 | 1.0000 | 16,980 | 72 | 114 | 9.0 | 121→148 MB |
| 100,000 | 1.0000 | 0.9984 | 102,250 | 175 | 308 | 9.0 | 133→358 MB |

Determinism: `determinism_same_seed=true` at every size.

### Gate evaluation

| Gate | Threshold | Result |
|---|---|---|
| Recall@10 vs exact (10K, 100K) | ≥ 0.95 | **PASS** (1.000, 0.9984) |
| 100K build wall time | < 2 min | **PASS** (102.25 s) |
| 100K peak RSS | < 1 GiB | **PASS** (358 MB) |

The 100K run is bounded and completed; no experiment exceeded its budget. The 100K wall
(218 s) includes exact ground-truth computation over 64 queries; the gate-relevant build
time is 102.25 s.

## 3. Robustness and lifecycle

| Check | Result |
|---|---|
| Invalid / zero / non-finite / dimension-mismatched vectors rejected | **PASS** (`TestANNRejectsInvalidVectors`) |
| Insertion order and duplicate keys handled | **PASS** (`TestANNInsertionOrderAndDuplicates`) |
| HNSW determinism under a fixed seed | **PASS** (`TestHNSWDeterminism`) |
| ANN lifecycle across index generations | **PASS** (`TestPhase2ANNLifecycleAcrossGenerations`) |
| Embedding-model version isolation | **PASS** (`TestPhase2EmbeddingModelVersionIsolation`) |
| Synthetic recall@10 (n=5000, M=64) | **PASS** (1.0000) |
| Hybrid ANN path correctness | **PASS** (`TestHybridANNPath`) |

## 4. M sweep and known limitations

`TestANNPhase1Matrix` (M ∈ {16,32,64}, seed 7) reproduced:

| n | M=16 R@1 / R@10 | M=32 R@1 / R@10 | M=64 R@1 / R@10 |
|---:|---|---|---|
| 1,000 | 0.9531 / 0.8641 | 0.9844 / 0.9625 | 1.0000 / 0.9953 |
| 5,000 | 0.7656 / 0.7609 | 0.9844 / 0.9844 | 1.0000 / 1.0000 |
| 10,000 | 0.8125 / 0.7969 | 0.9844 / 0.9844 | 1.0000 / 1.0000 |

- **Low `M` degrades recall** (M=16 ≈ 0.76–0.86 R@10). M=64 is the verified reference;
  lower `M` is an optional comparison only, never a default.
- **Flat space is pathological.** `TestANNRecallFlatSpace` reports ANN-vs-hashed-exact
  recall@10 = **0.000** (informational; the test passes). Near-orthogonal vectors with no
  cluster structure defeat graph navigation. This is a real limitation to flag before any
  ANN default on natural embeddings.

## 5. Reproduce

```
cd cli/beacon
BEACON_PHASE3_ANN=1 BEACON_PHASE3_ANN_SIZES=1000,10000,25000 BEACON_ANN_M=64 \
  go test ./internal/learning/ -run '^TestPhase3AANNvsExact$' -v -count=1
BEACON_PHASE3_ANN=1 BEACON_PHASE3_ANN_SIZES=100000 BEACON_ANN_M=64 \
  go test ./internal/learning/ -run '^TestPhase3AANNvsExact$' -v -count=1
BEACON_ANN_MATRIX=1 go test ./internal/learning/ -run TestANNPhase1Matrix -v -count=1
go test ./internal/learning/ -run 'TestANNRejectsInvalidVectors|TestHNSWDeterminism|TestANNInsertionOrderAndDuplicates|TestANNRecallSynthetic|TestHybridANNPath|TestPhase2ANNLifecycleAcrossGenerations' -v -count=1
```

Raw evidence: `RAW_EVIDENCE/ann_vs_exact_m64.txt`, `ann_vs_exact_100k.txt`,
`ann_correctness.txt`, `ann_correctness_extra.txt`, `ann_lifecycle.txt`.
