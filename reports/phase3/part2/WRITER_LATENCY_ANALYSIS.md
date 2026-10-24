# Phase 3B — Writer Latency Analysis

## 1. Reproduction (existing workload, churn ON, mixed)

`TestPhase2ConcurrencyStress`, two runs each, 10 s:

| Clients | read p50/p95/p99 µs | write p50/p95/p99 µs | errors | busy | stale |
|---:|---|---|---:|---:|---:|
| 8 | 2,091–2,183 / 7,712–9,068 / 25,141–26,988 | 9,330–10,202 / 85,563–85,689 / **141,729–183,826** | 0 | 0 | 0 |
| 16 | 2,446–2,832 / 10,469–12,839 / 31,338–59,895 | 16,813–17,919 / 340,518–345,818 / **950,913–1,255,010** | 0 | 0 | 0 |

This reproduces Phase 3A (8-client ≈ 132 ms, 16-client ≈ 848 ms). Zero errors, zero
busy events, zero stale reads at every level.

## 2. Steady state vs maintenance spikes

`TestPhase3BWriterLatency` separates the causes (10 s runs, 2 reps unless noted):

| Experiment | read p99 | write p50 | write p95 | write p99 | errors/busy |
|---|---:|---:|---:|---:|---:|
| 16 / mixed / churn **ON** | 3.5–5.6 ms | 4.0 ms | 183–232 ms | 746–1,036 ms | 0/0 |
| 16 / mixed / churn **OFF** | 6.0–6.4 ms | 4.1–4.8 ms | 232 ms | 843–936 ms | 0/0 |
| 16 / **write-only** / churn OFF | — | 3.1–4.0 ms | 55–58 ms | **131–134 ms** | 0/0 |
| 16 / **selective reads** / churn OFF | 5.2–15.3 ms | 5.1–5.7 ms | 184–234 ms | 935–942 ms | 0/0 |
| 16 / **read-only** / churn OFF | 15.7–17.9 ms | — | — | — | 0/0 |
| 8 / mixed / churn ON | 3.8–5.7 ms | 3.3–4.0 ms | 81–106 ms | 432–532 ms | 0/0 |
| 8 / mixed / churn OFF | 3.4–3.6 ms | 3.5–3.7 ms | 83–84 ms | 433–439 ms | 0/0 |

**Index rebuilds are not the cause:** churn ON and OFF are statistically identical at
both 8 and 16 clients. **CPU is not saturated:** the process used 154–199 % of 800 %
available. A 30 s bounded soak is stable (no growth), so this is steady-state, not a
warm-up or leak artifact:

| Soak | write p50 | write p95 | write p99 | RSS | CPU |
|---|---:|---:|---:|---:|---:|
| 8 clients, 30 s | 5.3 ms | 130 ms | 633 ms | 214 MB | 154 % |
| 16 clients, 30 s | 5.9 ms | 432 ms | 1,237 ms | 214 MB | 163 % |

## 3. Root cause: per-operation connection churn

Every store operation calls `s.db()`, which does `sql.Open(...)` and is followed by
`defer db.Close()` — a fresh connection pool per call, i.e. a physical SQLite
connection opened and closed per read/write. Two decisive measurements:

**Connection cost (16 concurrent goroutines, 5 s):**

| Pattern | p50 | p95 | p99 | ops in 5 s |
|---|---:|---:|---:|---:|
| per-op open + query + close | 3,564 µs | 5,996 µs | 7,683 µs | 21,472 |
| shared pool | **67 µs** | **304 µs** | 5,860 µs | 294,569 |

**Concurrent write probe (shared pool, 16 writers, 5 s), isolating fsync:**

| `synchronous` | p50 | p95 | p99 | writes in 5 s |
|---|---:|---:|---:|---:|
| FULL | 35 µs | 85 µs | **214 µs** | 115,162 |
| NORMAL | 30 µs | 73 µs | 187 µs | 131,663 |
| OFF | 28 µs | 69 µs | 175 µs | 132,926 |

With a shared pool, 16 concurrent writers reach p99 ≈ **214 µs** even at
`synchronous=FULL`; the store's per-op pattern reaches ≈ **940 ms**. `synchronous`
FULL/NORMAL/OFF differ by < 20 %, so fsync is not the cause. The block profile shows
`database/sql` connection-management frames and `runtime.selectgo` (idle pool opener),
not SQLite lock waits. The tail scales with the number of connection-churning
goroutines: write-only (16 churners) p99 132 ms, mixed (32 churners) p99 ≈ 940 ms.

**Conclusion.** The 16-client write tail is caused by per-operation connection pool
creation/destruction (physical connect/close, WAL-index `-shm` and file-lock churn),
amplified by concurrent readers. It is not rebuilds, not fsync, not CPU, not SQLite
write-lock serialization.

## 4. Why no fix was landed

The obvious fix — cache one `*sql.DB` per `Store` and add `Close()` — touches 11
`defer db.Close()` call sites and is coupled to the `schemaInitialized` / index-rebuild
mechanism (the churn and crash harnesses deliberately drop and recreate the derived
schema per `db()` call). Landing it without reworking that lifecycle risks the crash,
durability and index-rebuild guarantees. The measured evidence and the exact fix are
recorded here for Phase 3C.

## 5. Operational budget (unchanged)

- The ≤8-client budget is **retained**. On the existing Phase 2 harness, 8-client write
  p99 is 141–183 ms (< 250 ms) and 16-client is 951–1,256 ms (> 250 ms).
- On the heavier-read Phase 3B harness, 8-client write p99 is 432–633 ms, so the budget
  is workload-dependent and must be quoted with the workload.

## 6. Reproduce

```
cd cli/beacon
for c in 8 16; do for r in 1 2; do BEACON_PHASE2_STRESS=1 BEACON_PHASE2_CONCURRENCY=$c BEACON_PHASE2_SECONDS=10 \
  go test ./internal/learning/ -run TestPhase2ConcurrencyStress -v -count=1; done; done
BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=10 BEACON_PHASE3_W_REPS=2 \
  BEACON_PHASE3_W_CHURN=0 BEACON_PHASE3_W_MODE=mixed go test ./internal/learning/ -run TestPhase3BWriterLatency -v -count=1
BEACON_PHASE3_WRITER=1 go test ./internal/learning/ -run 'TestPhase3BConnectionMaterialised|TestPhase3BSynchronousCost' -v -count=1
```

Raw evidence: `RAW_EVIDENCE/writer_repro_existing.txt`, `writer_matrix.txt`,
`writer_isolation.txt`, `writer_soak.txt`, `writer_connection_materialised.txt`,
`writer_synchronous.txt`, `writer_time_16.txt`, `block16_top.txt`.
