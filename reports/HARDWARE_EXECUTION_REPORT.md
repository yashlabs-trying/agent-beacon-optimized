# Beacon V6 — Hardware Execution Report

Date 2026-10-21. Branch `feat/beacon-memory-engine-v6-production`.

## 1. Machines

| | Local laptop | RunPod pod |
|---|---|---|
| OS | WSL2 Ubuntu | Linux (container) |
| CPU | i5, 8 vCPU | AMD EPYC 7H12, 64 cores / 256 threads |
| RAM | 8 GB | 1007 GB (942 GB free) |
| GPU | none | RTX 3090, 24 GB VRAM |
| Disk | — | 30 GB overlay (1.2 GB used) |
| Go | 1.27 | installed 1.27 for this work |
| Python | — | 3.12.3, PyTorch (CUDA true), sentence-transformers 6.2.0 |

Pod access: SSH over exposed TCP (`root@213.192.2.119:40197`). Key registered via
the pod's `authorized_keys`.

## 2. What ran where

**Local laptop (CPU):** all V2–V5 work (lexical engine, trust, context, cache,
concurrency, the ANN recall fix) — see prior reports.

**Pod (CPU/RAM + GPU):**
- Correctness suite: PASS.
- Race suite: **PASS** (749.9 s).
- 1M lexical scaling: measured.
- Neural embedding sidecar (bge-small-en-v1.5, CUDA): quality + throughput.
- ANN build/recall/latency scaling (M=64 at 100K; M=16 at 1M).

## 3. GPU utilization

- Used only for embeddings. RTX 3090: **504 MiB VRAM**, GPU util ~0% between
  requests (bursty). HNSW and SQLite are CPU-only and never touched the GPU.
- Embedding throughput (bge-small, batch): 1→16.9 ms/text; 32→1.17; 256→0.62;
  1024→0.587; 4096→0.588. Steady state **~1700 embeddings/s**, ~0.59 ms/text.

## 4. Results

### Lexical, 1M memories (pod, measured)

DB 1.35 GB; seed 19.3 s; index build 91.1 s. Cold p50: selective 27.2 ms,
old-only 2.87 ms, no-match 1.69 ms, common 1.02 ms, empty 0.97 ms, mixed 1.19 ms.
Warm p50: old-only 0.91 ms, no-match 0.92 ms. (Absolute values differ from the
laptop; not compared across machines.)

### Neural vs hashed (labeled 17-case set)

| embedder | recall@1 | recall@5 | MRR | nDCG@5 |
|---|---|---|---|---|
| hashed (CPU) | 0.412 | 0.706 | 0.549 | 0.590 |
| **bge-small-en-v1.5 (GPU)** | **0.471** | **1.000** | **0.676** | **0.758** |

Neural closes the synonym/misleading gap the hashed embedder could not.

### ANN scaling (synthetic 128-dim, recall vs exact NN)

| n | M | build | recall@10 | ANN p50 | exact p50 | speedup | peak RSS |
|---|---|---|---|---|---|---|---|
| 100,000 | 64 | 505.9 s | **0.9953** | 1.20 ms | 59.2 ms | 49× | 1059 MB |
| 1,000,000 | 16 | 819.5 s | 0.4660 | 0.41 ms | 410.6 ms | 1000× | 3963 MB |

Build cost per vector: **M=64 ≈ 5 ms/node**, **M=16 ≈ 0.82 ms/node** (measured;
the per-node cost grows with n). 1M **M=64 ≈ 83 min**. At 1M, M=16 recall@10 falls
to 0.466, so **M=64 is required for the ≥0.95 target — but its build is the
scaling ceiling**, not query latency (which stays sub-millisecond).

## 5. Compute expenditure

- GPU active only for embeddings (seconds). Approx GPU-hours for this work: < 0.1.
- CPU/RAM: the pod was billed for its wall-clock lifetime; the 1M M=64 build was
  the dominant cost and was stopped after establishing the throughput ceiling.
- Estimated incremental GPU spend for the completed work: **negligible (<$1)**.

## 6. Tests that could not complete

- 1M ANN **M=64** (est. 85 min build) — stopped; throughput measured instead.
- Real coding-agent evaluation — no agent harness/model run yet.
- Crash-during-transaction fault injection — not implemented.
- Neural held-out set is 17 cases, not the 500 target.

## 7. Is a larger GPU necessary?

No. Embeddings use 0.5 GB of 24 GB; the engine is CPU/RAM-bound. The bottleneck
is **HNSW build throughput (single-threaded, CPU)**, which a bigger GPU would not
help. A faster/parallel ANN build or a different index is the real need.

## 8. Remaining blockers

1. HNSW build throughput (~200 nodes/s at M=64) — needs parallel construction or
   a faster implementation for 1M.
2. Neural held-out set size; agent evaluation; crash injection.
3. Pod disk 30 GB limits bundling models alongside large datasets.
