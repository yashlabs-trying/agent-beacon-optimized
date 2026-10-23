# Phase 5 — Scalability

Two kinds of scaling are reported separately: retrieval/selector scaling (measured) and
the agent-task matrix (measured on the 300 tasks). 1M and 32/64-client tiers are
**NOT TESTED** with the reason.

## 1. Context-selector scaling (measured, Phase 5 commit)

| n candidates | naive O(n²) | indexed | speedup |
|---:|---:|---:|---:|
| 20 | 0.19 ms | 0.17 ms | 1.1× |
| 100 | 1.33 ms | 0.69 ms | 1.9× |
| 500 | 52.9 ms | 12.2 ms | 4.3× |
| 1000 | 114.7 ms | 3.2 ms | **36.2×** |

Results identical (conflicts equal). The selector is sub-6 ms at n=1000.

## 2. Retrieval at scale (independent baselines, synthetic, 200 queries)

| Scale | BM25 p50 | FTS5 p50 | FTS5 p99 | FTS5 build |
|---:|---:|---:|---:|---:|
| 1K | 0.47 ms | 0.014 ms | 0.19 ms | 7 ms |
| 10K | 4.32 ms | 0.016 ms | 0.05 ms | 36 ms |
| 100K | 49.2 ms | 0.026 ms | 0.08 ms | 484 ms |

Beacon lexical is indexed (sub-3 ms at 1M, Phase 1). ANN 100K M=64 recall@10 0.9953,
build 102 s (Phase 3A).

## 3. Agent-task matrix (300 tasks × 5 arms)

| Suite | A | B | C | D | E |
|---|---:|---:|---:|---:|---:|
| A (100) | 0 | 75 | 60 | 60 | 79 |
| B (100) | 10 | 62 | 57 | 72 | 75 |
| C (100) | 20 | 92 | 92 | 92 | 92 |

No degradation as the suite grows; the per-suite differences are category-driven, not
size-driven.

## 4. Memory-history reliability (reused, Phase 4D)

10/50/100-session correction chains: the layer keeps only the current fact; untrusted
corrections never override; deterministic. **PASS.**

## 5. Not tested (honest)

| Tier | Reason |
|---|---|
| 1M memories (Beacon ANN M=64) | needs ~1.5 GB vectors + ~90 min build; WSL2 host 7.6 GiB |
| 32 / 64 concurrent clients | host limits |
| Disk-exhaustion recovery | not exercised |

## 6. Concurrency (reused, Phase 3C)

≤8 clients: write p99 ≤135 ms **PASS**; 16 clients: 744–795 ms **FAIL** (documented
SQLite single-writer ceiling; ≤8-client budget retained). The Phase 5 change adds no
writes, so it cannot affect this.

## Verdict

Retrieval/selector scaling 1K–100K: **PASS** (near-linear selector). 1M / 32–64 clients:
**NOT TESTED**. Concurrency ≤8: **PASS**; 16: **FAIL** (unchanged).
