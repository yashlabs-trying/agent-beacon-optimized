# Phase 4C — Scalability Results (Benchmark E)

## E1 — Memory corpus scaling (retrieval)

Reused from Phase 4A (`retrieval_baselines.py`), same hardware/query classes:

| Scale | BM25 p50 | FTS5 p50 | FTS5 p99 | FTS5 build |
|---:|---:|---:|---:|---:|
| 1K | 0.54 ms | 0.018 ms | 0.070 ms | 8 ms |
| 10K | 8.74 ms | 0.021 ms | 0.091 ms | 49 ms |
| 100K | 56.8 ms | 0.033 ms | 0.073 ms | 498 ms |

Beacon lexical is indexed and sub-3 ms at 1M (Phase 1). ANN 100K M=64 recall@10 0.9953,
build 102 s. **1M corpus: BLOCKED** — needs ~1.5 GB vectors + graph and ~90 min build;
the WSL2 host has 7.6 GiB and the pod run used ~4 GB RSS at 1M M=16. Not attempted.

The Phase 4C selection layer adds a **bounded** cost: it scans only the retrieved top-K,
so its O(K²) supersession scan is independent of corpus size. Measured selection cost at
K=8 is sub-millisecond on the dev corpus.

## E2 — Concurrency scaling

Reused from Phase 3C (storage engine unchanged; the new layer is read-only and adds no
writes):

| Clients | Write p99 | Status |
|---:|---:|---|
| 1 / 4 / 8 | ≤135 ms | **PASS** (< 250 ms) |
| 16 | 744–795 ms | **FAIL** (documented; ≤8-client budget retained) |
| 32 / 64 | not run | **BLOCKED** |

Read-heavy vs write-heavy: the agent workload is **read-dominant** (retrieval + selection);
write tail latency is the known SQLite single-writer limit and is **not** claimed solved.
Improved read scalability does not imply write scalability.

## E3 — Agent outcome scaling

**BLOCKED.** Running comparable model tasks at 1K/10K/100K memory-history sizes with
3 seeds is beyond the local compute budget for this phase. The bounded memory-history
test (`MEMORY_RELIABILITY.md`) covers correct-fact retention at 10/50/100 sessions, not
agent outcomes at corpus scale.

## Durability

Unchanged: full learning suite PASS (40.2 s), race subset PASS (15.7 s); crash/durability
suites from Phase 3/4A unchanged (the layer adds no persistence). SQLite durability
guarantees preserved.

## Verdict

E1 1K/10K/100K **PASS**, 1M **BLOCKED**; E2 ≤8 **PASS**, 16 **FAIL**, 32/64 **BLOCKED**;
E3 **BLOCKED**.
