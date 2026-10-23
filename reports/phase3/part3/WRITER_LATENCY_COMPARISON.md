# Phase 3C — Writer Latency Comparison

All runs on the same machine (WSL2 `lappiiii`, 8 vCPU, 7.6 GiB, Go 1.27.3), same
harnesses, same configuration. "Before" = Phase 3B commit `17a10b0f` (per-operation
pools). "After" = Phase 3C (per-Store pool). Three repetitions unless noted; medians and
ranges are reported rather than a single best run.

## 1. End-to-end workload (Phase 3B harness, mixed reads+writes)

| Config | Before write p50 / p95 / p99 | After write p50 / p95 / p99 | Before read p99 | After read p99 |
|---|---|---|---|---|
| 16 / mixed / churn OFF | 3.4–3.6 ms / 131–181 ms / **632–832 ms** | 2.0 ms / 106 ms / **535–632 ms** | 1.3–1.5 ms | **0.56–0.58 ms** |
| 16 / mixed / churn ON | 3.5–3.6 ms / 181 ms / **831–832 ms** | 2.1–2.8 ms / 130–182 ms / **732–735 ms** | 1.5–1.6 ms | 0.60–1.16 ms |
| 8 / mixed / churn OFF | 3.4–3.5 ms / 81 ms / **431–432 ms** | 1.8–2.1 ms / 55–81 ms / **232–332 ms** | 1.5–2.1 ms | 0.60–1.05 ms |
| 16 / write-only | 3.3 ms / 55 ms / **114–124 ms** | 3.9–4.2 ms / 84 ms / 184–186 ms | — | — |

## 2. End-to-end workload (Phase 2 canonical stress harness)

| Clients | Before write p99 | After write p99 | Before read p99 | After read p99 |
|---:|---|---|---|---|
| 8 | 149–199 ms | **118–135 ms** (PASS < 250 ms) | 7.9–22.3 ms | 8.3–9.1 ms |
| 16 | 945–1054 ms | **744–795 ms** (FAIL < 250 ms) | 32.8–38.5 ms | 15.9–47.8 ms |

Zero errors, zero busy events, zero stale reads at every level, before and after.

## 3. Isolated pool/operation microbenchmarks (16 clients)

| Probe | Before | After |
|---|---|---|
| Pool reuse, repeated `db()` (no query) | 4.6 µs/op | **0.0 µs/op** |
| Per-operation open+query+close vs pooled handle | perop 4,628 µs p50 / 10,978 µs p99; shared 88 µs p50 | perop_raw 4,728 µs p50 / 19,166 µs p99; **store_pooled 88 µs p50 / 421 µs p95 / 7,110 µs p99** |

The Store's pooled handle matches the shared-pool diagnostic: p50 improves ~54×, and
throughput rises from ~17k to ~250k operations per 5 s.

## 4. Schema initialization and shutdown cost

- Initialization is one `PRAGMA journal_mode=WAL` + `CREATE IF NOT EXISTS` set, run once
  per Store (previously re-checked on every `db()` call, though short-circuited by
  `schemaInitialized`). It is unchanged in content and now serialized by `schemaInitMu`.
- `Close()` closes the pool; it is idempotent. No measurable shutdown cost.

## 5. Steady state vs maintenance spikes

- Index-rebuild churn (dropping `index_ready` every 2 s) makes little difference:
  16/mixed churn ON p99 732–735 ms vs OFF 535–632 ms. Rebuilds are **not** the tail
  cause.
- A 30 s soak (Phase 3B) was stable; the tail is steady-state, not a warm-up or leak.

## 6. Remaining bottleneck (16 clients) — honest diagnosis

The 16-client write p99 target (< 250 ms) is **not met** after the fix (~744–795 ms on
the Phase 2 harness). It is not connection churn (fixed), not fsync, and not
checkpointing:

- **Single-writer phase profile** (300 sequential writes via the `writeCheckpoint` seam):
  total p50 358 µs; `after-begin` (BEGIN IMMEDIATE / lock acquire) p50 219 µs dominates,
  then insert 30 µs, FTS upsert 13 µs, generation 9 µs, commit 36 µs. The transaction
  body is cheap; the cost is acquiring the write lock.
- **Checkpoint probe** (16 writers, shared pool, 5 s): no readers → write p99 2.6 ms;
  with 16 concurrent full-scan readers → write p99 **1.8 s**. Setting
  `wal_autocheckpoint` to 0 or 10000 did **not** help (p99 2.37 s / 2.50 s), so WAL
  checkpointing is not the cause; reader CPU/transaction contention is.

Conclusion: SQLite's single-writer model serializes 16 concurrent `BEGIN IMMEDIATE`
transactions, and concurrent readers amplify the tail. There is no minimal, safe fix
that preserves the transaction and durability guarantees (a shorter `busy_timeout` would
surface `SQLITE_BUSY` errors instead of waiting; batching/`BEGIN CONCURRENT` would change
semantics). This is documented as the remaining limitation.

## 7. Operational budget

The documented **≤8-client** limit is **retained**:
- Phase 2 harness 8-client write p99 is now **118–135 ms** (< 250 ms), a clear PASS.
- 16-client remains > 250 ms.

No product default was changed to improve a benchmark.

## 8. Reproduce

```
cd cli/beacon
# before/after end-to-end
for c in 8 16; do for r in 1 2; do BEACON_PHASE2_STRESS=1 BEACON_PHASE2_CONCURRENCY=$c BEACON_PHASE2_SECONDS=10 \
  go test ./internal/learning/ -run TestPhase2ConcurrencyStress -v -count=1; done; done
BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=10 BEACON_PHASE3_W_REPS=3 \
  BEACON_PHASE3_W_CHURN=0 BEACON_PHASE3_W_MODE=mixed go test ./internal/learning/ -run TestPhase3BWriterLatency -v -count=1
BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=5 \
  go test ./internal/learning/ -run 'TestPhase3BConnectionMaterialised|TestPhase3BConnectionCost' -v -count=1
BEACON_PHASE3C_PROBE=1 go test ./internal/learning/ -run 'TestPhase3CCheckpointProbe|TestPhase3CWritePhaseProfile' -v -count=1
```

Raw evidence: `RAW_EVIDENCE/before_writer.txt`, `after_writer.txt`,
`checkpoint_probe.txt`, `write_phase_profile.txt`.
