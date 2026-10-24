# Phase 3A — Reliability, Security, and Concurrency (Workstream E)

**Status: PASS on all focused gates except the 16-client write tail-latency target, which
is CONDITIONAL (supported budget = ≤8 clients).**

## 1. Static and build gates

| Check | Result |
|---|---|
| `go version` | go1.27.3 linux/amd64 |
| `go build ./...` | **PASS** (exit 0) |
| `go vet ./...` | **PASS** (exit 0) |
| `gofmt -l .` | **PASS after fix** — 2 inherited files flagged at `c9275d84`, reformatted (whitespace only) |
| `git diff --check` | **PASS** (exit 0) |
| `go test ./...` (CLI module) | **PASS** after building the real hooks binary (see §6) |

## 2. Concurrency (disposable DBs, 10 s windows)

| Clients | Reads | Writes | Supersedes | errors | busy | stale | read p50/p95/p99 µs | write p50/p95/p99 µs |
|---:|---:|---:|---:|---:|---:|---:|---|---|
| 1 | 3,370 | 895 | 211 | 0 | 0 | 0 | 460 / 818 / 1,204 | 1,181 / 1,984 / 2,776 |
| 4 | 4,203 | 1,114 | 262 | 0 | 0 | 0 | 1,034 / 2,284 / 3,489 | 3,621 / 11,147 / 36,291 |
| 8 | 3,871 | 1,024 | 242 | 0 | 0 | 0 | 1,884 / 4,077 / 6,466 | 7,591 / 58,041 / 132,770 |
| 16 | 3,784 | 996 | 235 | 0 | 0 | 0 | 2,273 / 5,465 / 28,141 | 11,764 / 337,090 / **848,306** |

- **Zero errors, zero busy events, zero stale reads** at every level.
- Goroutines stable (3 → 2), heap stable (1–2.5 MB alloc), no sustained growth.
- **Write tail latency** reproduces the known Phase 2 behaviour: p99 rises from 2.8 ms
  (1 client) to 132 ms (8 clients) to **848 ms (16 clients)**.
  - Gate: steady-state read/write p99 < 250 ms at the supported client count.
  - **8 clients PASS** (write p99 132 ms). **16 clients FAIL** (write p99 848 ms).
  - Per the plan, publish the lower operational budget: **≤8 clients**. This is not hidden
    in an average — the 16-client mean (61 ms) masks a 848 ms p99.
- `TestPhase2CacheInvalidationUnderWrites` (read-after-write consistency under a dropped
  index/cache): **PASS** at every client count.

## 3. Durability, crash, and SQLite recovery

| Check | Result |
|---|---|
| `TestPutMemoryProcessCrashRecovery` (after-begin/authoritative/index/generation/before-commit/after-commit) | **PASS** |
| `TestPhase2CrashDurability` (tx_begin_pre, tx_begin_post, memory_insert, fts_update, generation_update, pre_commit, post_commit, post_return) | **PASS** |
| `TestPhase2CrashDuringIndexRebuild` | **PASS** |
| `TestPhase2CrashDuringMigration` | **PASS** |
| `TestPhase2CrashDuringSupersession` | **PASS** |
| `TestPhase2RepeatedRecovery` (idempotent recovery) | **PASS** |
| `TestPhase2FaultHookIsNilInProduction` | **PASS** |
| `TestPutMemoryWaitsForBoundedWriteContention` (SQLite busy handling, 11 s hold) | **PASS** |
| SQLite `PRAGMA integrity_check` after each crash | **PASS** |

No acknowledged write was lost. **These are process-crash injections, not power-loss
tests** — no power-loss durability is claimed.

## 4. Security and trust boundaries

| Check | Result |
|---|---|
| Cross-scope leakage (lexical / hybrid exact / hybrid ANN / path traversal) | **PASS** |
| Unapproved evidence not visible; approved visible | **PASS** |
| Hostile stored content (SQL injection, path traversal, 200 KB oversized, unicode-bidi) | **PASS** |
| Adversarial contradiction matrix (65 cases) | precision 1.000, recall 0.758, FPR 0.000 |
| Trust annotation latency (n=500) | 5.01 ms total, 10.0 µs/item |
| Approval boundary preserved (no automatic approval/deletion) | **PASS** |

No synthetic hostile payload bypassed the storage trust boundary. **This does not
establish real-world prompt-injection resistance.**

## 5. Race detector

| Package | Result | Time |
|---|---|---|
| `internal/learning` (`-race`) | **PASS** | 571.69 s |
| `internal/hnsw` (`-race`) | **PASS** | 2.99 s |

## 6. Repository-wide CLI suite

`go test -p 2 -count=1 -timeout 30m ./...` from `cli/beacon`:

- First run: only failures were `no hooks binary embedded` (a **placeholder** artifact, not
  a code defect). The `learning` (48.1 s) and `hnsw` packages passed.
- After `make build-hooks-current` (builds the real `beacon-hooks` binary into the
  gitignored `internal/embedded/hooks.bin`), re-running the previously failing packages —
  `cmd`, `endpoint/dashboard`, `endpoint/harness`, `endpoint/hooks`, `endpoint/inventory` —
  all **PASS**.
- **Conclusion: the CLI module suite passes.** The initial failures were environmental
  (missing embedded hooks binary), reproduced against the frozen baseline, and unrelated
  to V6.

## 7. Reproduce

```
cd cli/beacon
go build ./... && go vet ./... && gofmt -l .
go test -race -p 2 -count=1 -timeout 40m ./internal/learning/ ./internal/hnsw/
make build-hooks-current
go test -p 2 -count=1 -timeout 30m ./...
for c in 1 4 8 16; do
  BEACON_PHASE2_STRESS=1 BEACON_PHASE2_CONCURRENCY=$c BEACON_PHASE2_SECONDS=10 \
    go test ./internal/learning/ -run 'TestPhase2ConcurrencyStress|TestPhase2CacheInvalidationUnderWrites' -v -count=1
done
go test ./internal/learning/ -run 'TestPhase2Crash|TestPhase2RepeatedRecovery|TestPhase2FaultHook|TestPutMemoryProcessCrashRecovery|TestPutMemoryWaitsForBoundedWriteContention' -v -count=1
```

Raw evidence: `RAW_EVIDENCE/build_vet_fmt.txt`, `gofmt_before.txt`, `gofmt_after.txt`,
`concurrency_{1,4,8,16}.txt`, `durability_crash.txt`, `security_trust.txt`,
`race_learning_hnsw.txt`, `full_cli_suite.txt`, `full_cli_suite_hooksfixed.txt`.
