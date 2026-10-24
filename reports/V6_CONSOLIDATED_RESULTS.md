# Beacon V6 — Consolidated Results

All values measured on the machines in `HARDWARE_EXECUTION_REPORT.md`, unless
marked otherwise. Baseline `99c70908`.

## Comparison

| Metric | Original Beacon | V4 | V5 | V6 (this) |
|---|---|---|---|---|
| Lexical recall@5 (17-case) | 0.412 | 0.412 | 0.412 | 0.412 (unchanged) |
| Hashed semantic recall@5 | n/a | 0.706 | 0.706 | 0.706 |
| **Neural (bge-small) recall@5** | n/a | n/a | n/a | **1.000** |
| Neural recall@1 | n/a | n/a | n/a | **0.471** (hashed 0.412) |
| ANN recall@10 (100K, M=64) | n/a | 0.17 (rejected) | 0.975 (20K) | **0.9953** (100K) |
| ANN search p50 (100K) | n/a | n/a | 1.20 ms | 1.20 ms |
| Exact vector search p50 (100K) | n/a | 513 ms | 38 ms | 38 ms (synthetic) |
| ANN speedup vs exact (100K) | n/a | n/a | ~72× | 49× |
| Lexical 1M old-only p50 | 3,387 ms | 1.60 ms | 1.60 ms | 1.60 ms (laptop); 2.87 ms (pod) |
| Lexical 1M no-match p50 | 3,263 ms | 0.61 ms | 0.61 ms | 0.61 ms (laptop); 1.69 ms (pod) |
| Context tokens (17 q) | 2,140 | 1,242 | 1,242 | 1,242 |
| ANN build (100K, M=64) | n/a | n/a | n/a | 506 s |
| Embedding throughput | n/a | n/a | n/a | ~1,700/s (0.59 ms/text) |
| GPU VRAM (embeddings) | n/a | n/a | n/a | 504 MiB |
| Agent task success | NOT MEASURED | NOT MEASURED | NOT MEASURED | NOT MEASURED |

## Findings

- **Neural embeddings work and improve quality**: recall@5 0.706 → 1.000 on the
  labeled set, on GPU, offline, ~1,700 embeddings/s, 0.5 GB VRAM.
- **ANN recall is fixed and validated at 100K**: 0.9953 (M=64), 49× faster than
  exact. The V5 fix (canonical layer search) is confirmed at scale.
- **The ANN scaling ceiling is build time and M, not query time.** Measured:
  100K M=64 → recall@10 0.9953, build 506 s; 1M M=16 → recall@10 **0.4660**,
  build 820 s, search 0.41 ms vs exact 411 ms (1000×), 3.96 GB RSS. Build cost is
  ~5 ms/node at M=64 and ~0.82 ms/node at M=16 (single-threaded), so 1M M=64 is
  ~83 min. At 1M, M=16 recall collapses below target; **M=64 is required but its
  build is the blocker.** Query latency stays sub-millisecond.
- **Lexical engine unchanged and still fast at 1M** on the pod.

## Production acceptance matrix

| Gate | Status | Basis |
|---|---|---|
| A Correctness | **PASS** | differential + full suite on laptop and pod |
| B Durability | **CONDITIONAL PASS** | index-drop recovery tested; crash-mid-transaction not injected |
| C Security | **CONDITIONAL PASS** | scope isolation verified; prompt-injection harness not built |
| D Retrieval quality | **PASS** | neural recall@5 1.000 > 0.706; ANN recall 0.9953 ≥ 0.95 |
| E Scale | **CONDITIONAL PASS** | 1M lexical PASS; 1M ANN measured only at M=16 (M=64 build 85 min) |
| F Performance | **PASS** | no measured regression on common queries/writes |
| G Reliability | **CONDITIONAL PASS** | `-race` PASS on both; soak/4–8 h not run |
| H Agent value | **NOT TESTED** | no agent harness run |
| I Operational readiness | **NOT TESTED** | deployment/rollback docs partial |
| J Reproducibility | **PASS** | tarball + scripts + commands recorded |

## Verdict

**RELEASE CANDIDATE — BLOCKERS REMAIN.** The engine is faster (lexical), more
accurate (neural + corrected ANN), and reliable under tested conditions, but
agent value is unmeasured and ANN build does not scale to 1M at M=64 within a
reasonable time. Do not claim production readiness.

## What V6 added (code)

- `internal/hnsw` vendored + fixed (V5), instrumented build (V6).
- `search_embed_http.go` — opt-in neural `Embedder` (local sidecar).
- `embed_sidecar.py` — local sentence-transformers sidecar (GPU, offline).
- `neural_test.go` — neural-vs-hashed quality comparison.
- ANN benchmark now reports build time, recall@10, and peak RSS per size.
