# Beacon V6 — Phase 3C Executive Results

Two tasks: implement and validate a persistent per-`Store` SQLite pool, and run real
BGE-small neural retrieval if a local model/runtime exists.

- Date: 2026-10-23 (Asia/Kolkata)
- Branch: `phase3c/beacon-v6-verification` (isolated worktree `/home/<user>/work/phase3c`)
- Parent: Phase 3B `17a10b0fa5966d34e476ddf71371aa5b1bc490dc`
- Ancestry verified: `99c70908` → `209a987b` → `c9275d84` → `c8a6749b` → `17a10b0f` → Phase 3C
- Frozen refs unchanged: integration `209a987bc96c29b5e5d811a794d0357f20214463`,
  prep `c9275d84185fc9d204429f913d516ff941d76be3`, baseline
  `99c709081ad49da1c5ade205929b61b19360ed9b`
- Fixtures re-verified `LOCK OK`: retrieval `FADF313E…114A`, contradiction `B3B4CAC0…8E84`,
  agent tasks `409195B0…C530`
- Host: WSL2 (`lappiiii`), 8 vCPU, 7.6 GiB, Go 1.27.3 linux/amd64

## 1. Gate table

| # | Gate | Result |
|---|---|---|
| P1 | Per-Store persistent `*sql.DB` pool (no per-op pool) | **PASS** |
| P2 | `Store.Close()` releases the pool; idempotent; later use fails with `ErrStoreClosed` | **PASS** |
| P3 | Race-free, idempotent schema init; no partial-schema exposure | **PASS** |
| P4 | Failed initialization leaks no handle | **PASS** |
| P5 | Multiple Stores independent; no shared incorrect state | **PASS** |
| P6 | Public behaviour, persistence, transactions, error handling preserved | **PASS** |
| W1 | 8-client write p99 < 250 ms | **PASS** (118–135 ms) |
| W2 | 16-client write p99 < 250 ms | **FAIL** (744–795 ms) — root cause documented |
| W3 | Zero unexpected errors / busy / stale reads | **PASS** (0/0/0) |
| W4 | Reads improved vs Phase 3B | **PASS** (p50 ~4×, p99 ~2.3×) |
| W5 | Pool reuse microbenchmark | **PASS** (p50 ~54×; 0.0 µs/op reuse) |
| W6 | ≤8-client operational budget retained | **PASS** |
| N1 | Real BGE-small neural retrieval | **BLOCKED** |
| V1 | gofmt / build / vet | **PASS** |
| V2 | Full learning package suite | **PASS** (37 s) |
| V3 | `go test -race` learning + hnsw | **PASS** (443 s / 3.8 s) |
| V4 | Crash / durability / security / contradiction regression | **PASS** |
| V5 | Repository-wide CLI suite | **PASS** (exit 0, real hooks binary) |
| D1 | Phase 3B decisions preserved (opt-in behaviours) | **PASS** |

## 2. What changed

Production (`cli/beacon/internal/learning`):
- `store.go`: `Store` gains `mu`/`dbPool`/`closed`; `db()` returns a lazily created,
  reused pool; new `openLocked`, `Close`, `ErrStoreClosed`; path normalization moved to
  `Open`; 8 `defer db.Close()` removed; `PutMemory` index-failure path calls
  `markIndexUnhealthy`.
- `search.go`: `ensureSchemaOnce` double-checks under `schemaInitMu`; `markIndexUnhealthy`
  re-ensures the schema directly.
- `search_semantic.go`: `OpenSemantic` normalizes the path; 2 `defer db.Close()` removed.

Tests: ownership audit across 12 test files (helpers/churn no longer close the shared
pool) + new `phase3c_pool_test.go` (7 lifecycle tests) + `phase3c_probe_test.go`
(diagnostics). The Phase 3B writer harness now closes each Store per repetition.

No retrieval, trust, ANN, or token-budget code changed. No defaults changed.

## 3. Writer latency (before → after, same machine/harness/config)

| Metric | Phase 3B | Phase 3C | Change |
|---|---|---|---|
| Read p50 (16 mixed) | ~463 µs | ~112 µs | ~4× better |
| Read p99 (16 mixed) | 1.3–1.5 ms | 0.56–0.58 ms | ~2.3× better |
| 8-client write p99 (Phase 2) | 149–199 ms | **118–135 ms** | PASS |
| 16-client write p99 (Phase 2) | 945–1054 ms | **744–795 ms** | ~25% better, still FAIL |
| 16 mixed write p95 | 131–181 ms | 106 ms | better |
| Pooled handle p50 (16 clients) | 4,628 µs (per-op) | **88 µs** | ~54× |

**Remaining 16-client bottleneck (diagnosed, not hidden):** the per-op connection churn
is gone, but 16 concurrent `BEGIN IMMEDIATE` writers still serialize on SQLite's single
write lock; concurrent readers amplify the tail via CPU/transaction contention. A
single-writer phase profile shows the lock-acquire phase dominates (219 µs of a 358 µs
transaction), and a checkpoint probe shows 16 full-scan readers push write p99 from
2.6 ms to 1.8 s while disabling `wal_autocheckpoint` does not help. No minimal, safe fix
preserves the durability/transaction guarantees. The ≤8-client budget is retained.

## 4. Neural

BLOCKED. The Mac is unreachable and WSL2 has no `torch`/`sentence-transformers` and no
`BAAI/bge-small-en-v1.5` cache. The locked fixture and harness are ready; the exact
prerequisite and run command are in `NEURAL_RETRIEVAL_RESULTS.md`.

## 5. Validation and resources

| Check | Result | Time |
|---|---|---|
| `gofmt -l .` | clean | — |
| `go build ./...` / `go vet ./...` | pass | — |
| `go test ./internal/learning/` | pass | 37.3 s |
| `go test -race ./internal/learning/ ./internal/hnsw/` | pass | 443 s / 3.8 s |
| Crash + durability + security + contradiction regression | pass | 11.5 s |
| `go test ./...` (CLI module) | pass (exit 0) | 77 s |

Bounded; no 1M ANN run, no paid workload. Peak RSS during writer runs ≈ 33 MB; CPU
154–199 % of 800 % (not CPU-bound).

## 6. Remaining risks and recommended next step

1. **16-client write p99 (~744–795 ms)** remains above target; keep ≤8 clients. A future
   fix would need a different write-concurrency strategy (e.g. write batching or a
   `BEGIN CONCURRENT`-style path) with its own durability analysis.
2. **WAL growth** with a persistent pool reaches the auto-checkpoint threshold (~4 MB) and
   more under sustained readers; durable and integrity-checked, but worth monitoring.
3. **Neural retrieval** still unverified; run on the Mac.
4. Keep neural, ANN, hybrid, token budgeting, and cross-kind contradiction detection
   **opt-in**.
5. The six-task paired agent evaluation remains a separate phase.
