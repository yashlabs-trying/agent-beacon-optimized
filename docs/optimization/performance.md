# Performance

Measured, reproducible results for the optimization work. Every number names its
source report and the commit it was measured on. Numbers that cannot be reproduced on
current hardware are labeled **historical**.

> **Read this first.** Beacon is a memory-retrieval and storage engine. These are
> **microbenchmarks and storage primitives**. None of them demonstrates that an agent
> becomes more accurate or more successful — see [limitations](limitations.md). The
> one model-backed agent-task dataset in this repo (a 56-task pilot) is labeled
> historical and is not independent evidence.

## At a glance

| Metric | Before | After | Status | Source |
|---|---|---|---|---|
| **16-client write p99** (ext4, 8 s mixed) | 634 ms | **48–54 ms** | reproducible | `reports/v7-hardening/D_DATABASE_CONCURRENCY_FIX.md` |
| **`SearchQuestion` allocs/op** (warm, 1k store, CandidateK 50) | 2,007 | **875** (−56.4%) | reproducible | `reports/v7-hardening/RETRIEVAL_ALLOCATION_FIX.md` |
| **`SearchQuestion` bytes/op** | 208,054 | **130,495** (−37.3%) | reproducible | `reports/v7-hardening/RETRIEVAL_ALLOCATION_FIX.md` |
| **Indexed no-match search** (scaling with N) | O(N): 8.05 → 46 ms | **81 → 142 µs** (flat) | reproducible | `reports/security/00_EXECUTIVE_SUMMARY.md` |
| **Cold healthy-store retrieval** | V7 43.7 ms | **20–36 ms** (≈ original 20–65 ms) | reproducible | `reports/v7-hardening/FINAL_STABILIZATION_REPORT.md` |
| Peak RSS (16/32-client writer probe) | 28 MB | ~25–30 MB | reproducible | `reports/v7-hardening/THREE_REMAINING_PROBLEMS_FINAL.md` |

Workload, units, and evidence links for each are detailed below.

---

## 1. Write concurrency

**Workload:** 300 seeded memories, sustained mixed read/write for 8 s at C clients,
database on local ext4 (`/tmp`). Host: WSL2, 8 vCPU, 7.6 GiB, Go 1.27.

**Root cause:** `_txlock=immediate` + 30 s busy_timeout; 16 connections busy-poll
SQLite's single write lock, producing a super-linear tail.

**Fix:** process-level `writeMu sync.Mutex` serializes in-process writers around the
write transaction. Durability unchanged (WAL, `synchronous` untouched).

| Clients | write p50 | write p95 | write p99 | read p99 | errors/busy |
|---:|---:|---:|---:|---:|---:|
| 1 | 1.1 ms | 2.7 ms | 4.9 ms | 0.57 ms | 0 / 0 |
| 4 | 5.6 ms | 12.4 ms | 21.8 ms | 0.81 ms | 0 / 0 |
| 8 | 13.4 ms | 25.7 ms | 33.3 ms | 0.83 ms | 0 / 0 |
| **16** | 27.1 ms | 40.7 ms | **48.1 ms** | 0.82 ms | 0 / 0 |
| 32 | 51.0 ms | 83.9 ms | 94.3 ms | 0.77 ms | 0 / 0 |

Before → after at 16 clients: **634 ms → 48 ms** (repeat: 50 ms). Write-heavy 16-client
p99 **16.4 ms**, ~2,011 writes/s. The median *rises* (2.9 → 27 ms) because writes now
queue; the tail collapses and net throughput improves.

**Filesystem caveat (must be stated):** on a synchronized/network mount
(WSL `/mnt/c`, OneDrive) the same code shows 16-client p99 ≈ 1.34 s because each commit
fsyncs at ~1 s. **The production database must live on a local filesystem.** This is
detected and warned about, not silently changed. See
[configuration](configuration.md#database-placement).

**Not tested:** 64 clients and above; 1M/10M memory scale.

## 2. Retrieval allocations

**Workload:** warm `SearchQuestion`, 1,000-memory store, CandidateK 50, 300 iterations.
Allocation counts are stable on this host; wall-clock is not (SQLite/I/O bound).

| Metric | Phase C start | After Phase C | After Phase 2 |
|---|---:|---:|---:|
| allocs/op | 2,099 | 2,007 | **875 (−56.4%)** |
| bytes/op | 248,382 | 207,988 | **130,495 (−37.3%)** |

`ListMemories` (Limit 8), control: ~58 µs, 64 allocs — unchanged.

**Why the comparison to `ListMemories` is not a ratio:** question-aware search
deliberately considers CandidateK=50 candidates and runs trust/supersession/injection
over them. The extra work is the intelligence layer, not a defect.

## 3. Database placement

| Workload (16 clients, mixed, 8 s) | DB filesystem | write p50 | write p99 |
|---|---|---:|---:|
| pre-fix | ext4 (`/tmp`) | 2.9 ms | 634 ms |
| pre-fix | OneDrive (`/mnt/c`) | 208.9 ms | 7,762 ms |

The synchronized mount is ~12× worse at p99 and ~70× worse at p50 — a deployment
constraint, not a code defect. `BEACON_MEMORY_DIR` + `CheckPlacement` + `MigrateDatabase`
(see [configuration](configuration.md)) let operators place the store correctly and
migrate safely.

## 4. Historical results (not reproducible on current hardware)

These informed the design but **must not be presented as current validation**:

| Result | Source | Why historical |
|---|---|---|
| Retrieval recall@5: lexical 0.167, hashed-semantic 0.875, BGE-small 0.979 | `reports/v7-hardening/G_ANN_AND_NEURAL.md` (Phase 4A) | sidecar model not present in current env |
| ANN 100K recall@10 0.9953, index build 102 s | Phase 3A (in `G_ANN_AND_NEURAL.md`) | not re-run |
| 300-task: original 229/300 vs V6 intelligent 224/300 | `reports/security/`-adjacent; Phase 5 | frozen evaluation; equivalence |
| 56-task V7 pilot: original 40/56 vs V7 51→56/56 | `reports/v7/V7_EVIDENCE_AND_VERDICT.md` | **7 repeated mechanism families**, not 56 real workflows |

The 56-task pilot's per-row McNemar p-value was 0.000977, but a **family-level sign
test gives p=0.5** because only two of seven families improved. Treat it as design
evidence, not credible superiority.

## Reproduce

See [benchmarks](benchmarks.md) for exact commands and seeds.
