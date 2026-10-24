# Phase 4B — Scale & Reliability Report

Scale/concurrency/fault testing is **separate** from answer quality. Phase 4B reuses the
Phase 3/4A verified suites rather than duplicating them, and adds the bounded synthetic
scale sweep from Phase 4A. The 1M tier is **BLOCKED** on this hardware.

## 1. Stored-memory scale (retrieval only, independent baselines)

From Phase 4A `RETRIEVAL_BASELINE_COMPARISON.md` (WSL2, 8 vCPU, 7.6 GiB):

| Scale | BM25 p50 | FTS5 p50 | FTS5 p99 | FTS5 build |
|---:|---:|---:|---:|---:|
| 1K | 0.54 ms | 0.018 ms | 0.070 ms | 8 ms |
| 10K | 8.74 ms | 0.021 ms | 0.091 ms | 49 ms |
| 100K | 56.8 ms | 0.033 ms | 0.073 ms | 498 ms |

Beacon lexical is indexed and sub-3 ms at 1M (Phase 1: 1.60 ms old-only p50). ANN:
100K M=64 recall@10 0.9953, build 102 s; 1M M=64 build ≈83 min (extrapolated).

**1M tier: BLOCKED.** Measured requirement: 1M × 384-dim float32 ≈ 1.5 GB vectors plus
graph overhead; the WSL2 host has 7.6 GiB total, and the Phase 1 pod run needed ~4 GB
RSS at 1M M=16. A safe 1M M=64 run needs ≥8 GB free and ~90 min build. Not attempted.

## 2. Concurrency (Phase 3C, WSL2)

| Clients | Write p99 | Status |
|---:|---:|---|
| ≤8 | 118–135 ms | **PASS** (< 250 ms) |
| 16 | 744–795 ms | **FAIL** (≥250 ms) |
| 32/64 | not run | **BLOCKED** |

Supported operational budget retained: **≤8 clients**. Root cause (per-op pool churn) was
fixed in Phase 3C; the residual 16-client tail is SQLite single-writer serialization.

## 3. Fault injection / recovery (Phase 3/4A, all PASS)

| Fault | Test | Result |
|---|---|---|
| Crash at 7 tx points | `TestPhase2CrashDurability` | PASS (0 write loss) |
| Crash during index rebuild / migration / supersession | `TestPhase2CrashDuring*` | PASS |
| Process kill mid-write | `TestPutMemoryProcessCrashRecovery` | PASS (6 sub-cases) |
| Reopen after interrupted startup | `TestPhase2ReopenAfterInterruptedStartup` | PASS |
| Corrupt row surfaced | `TestPhase2CorruptRowSurfacedNotSilent` | PASS |
| Index rebuild after deletion | `TestPhase2IndexRebuildAfterDeletion` | PASS |

## 4. Context budgets

Budget variants 256/512/1024/2048 tokens were **NOT** swept as a matrix in Phase 4B. The
`context` dev case (5 competing flags) passed for all semantic arms at the default budget;
the token-budgeting selector remains opt-in and loses 3/109 findings (Phase 3A/3B).
**Gate: BLOCKED (matrix not run).**

## 5. Summary

| Dimension | Result |
|---|---|
| Retrieval scale 1K/10K/100K | PASS |
| Retrieval scale 1M | BLOCKED (RAM/time) |
| Concurrency ≤8 | PASS |
| Concurrency 16 | FAIL (p99) |
| Concurrency 32/64 | BLOCKED |
| Crash/fault recovery | PASS |
| Context-budget matrix | BLOCKED |
