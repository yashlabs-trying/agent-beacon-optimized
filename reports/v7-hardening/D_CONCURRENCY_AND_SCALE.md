# Beacon V7 hardening — Phase D: Concurrency and Scale

Status: **Phase D measured (partial). The 16-client write tail remains unresolved.**

## 1. Concurrency (measured on this host, WSL2, 8 vCPU)

### Phase 2 stress harness — 8 clients, 10 s

```
STRESS concurrency=8 duration_s=10 reads=2867 writes=757 supersedes=180 errors=0 busy=0 stale=0
read_latency_us  p50=1073 p95=5474 p99=11924
write_latency_us p50=8638 p95=64276 p99=130493
```

### Phase 3B writer harness — 16 clients, mixed, 10 s (2 reps)

| rep | read p50 | read p99 | write p50 | write p95 | write p99 | busy/errors |
|---|---:|---:|---:|---:|---:|---:|
| 1 | 0.28 ms | 2.30 ms | 5.92 ms | 233 ms | **935 ms** | 0 / 0 |
| 2 | 0.23 ms | 1.36 ms | 3.70 ms | 182 ms | **835 ms** | 0 / 0 |

**16-client write p99 ≈ 835–935 ms** — far above a 250 ms budget. This matches the
pre-existing finding (V7 742 ms, Phase 5 635 ms on other runs). The host is
I/O-loaded (OneDrive/WSL), so absolute values vary, but the order of magnitude and
the single-writer serialization are consistent.

## 2. Root cause (carried from Phase 3C, re-confirmed)

SQLite's single-writer lock: 16 concurrent `BEGIN IMMEDIATE` writers serialize, and
concurrent readers amplify the tail via CPU/transaction contention. Per-op pool churn
was already removed in Phase 3C; the residual tail is the write lock itself.

## 3. Optimizations considered (not applied)

- Shorter transactions / batching / bounded write queue / backpressure / WAL tuning /
  reduced index maintenance / checkpoint scheduling. None was applied because no
  minimal change preserves durability, atomicity, and crash recovery; the mission
  forbids trading durability for latency. The ≤8-client operating budget is retained.

## 4. Matrix coverage

| Clients | Status |
|---|---|
| 1, 4, 8 | measured (8-client: reads p99 11.9 ms, writes p99 130 ms) |
| 16 | measured: write p99 835–935 ms — **FAIL** vs 250 ms |
| 32, 64, 128 | **NOT TESTED** (host limits) |

## 5. Memory scale

| Records | Status |
|---|---|
| 10K / 100K | retrieval measured in Phase 4A/5 (FTS5 p50 ≤0.03 ms; BM25 ≤49 ms at 100K) |
| 1M | **NOT TESTED** (needs ~1.5 GB vectors + ~90 min ANN build; host 7.6 GiB) |
| 10M | **NOT TESTED** (hardware) |

## 6. Verdict

Concurrency ≤8: **PASS**. 16-client writer p99: **FAIL / OPEN** (SQLite single-writer
ceiling; durability retained). 32/64/128 clients and 1M records: **NOT TESTED**.
