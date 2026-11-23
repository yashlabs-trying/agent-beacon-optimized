# Beacon V6 — Phase 3B Executive Results

Resolution / characterization of the four Phase 3A blockers. No locked fixtures
were changed, no defaults were flipped, no paid compute or GPU was used, and the
frozen baseline / integration / Phase 3 prep / Phase 3A commit were not modified.

- Date: 2026-10-23 (Asia/Kolkata)
- Branch: `phase3b/beacon-v6-verification` (isolated worktree `/home/<user>/work/phase3b`)
- Base: Phase 3A commit `c8a6749bb91d6cc85cb317ba0c75fcb5f23305ad`
- Unchanged frozen refs: integration `209a987b…`, prep `c9275d84…`, baseline `99c70908…`, tag `freeze/beacon-v6-integration-verified-209a987b`
- Fixtures re-validated with `validate_lock.py`: all `LOCK OK` (retrieval `FADF313E…`, contradiction `B3B4CAC0…`, agent tasks `409195B0…`)
- Host: WSL2 (`lappiiii`), Linux 6.18.33.2-microsoft-standard-WSL2, 8 vCPU, 7.6 GiB; Go 1.27.3 linux/amd64

## 1. Gate table

| # | Blocker / gate | Phase 3A | Phase 3B | Status |
|---|---|---|---|---|
| C1 | Contradiction precision (locked, default) | 1.000 | **1.000** | PASS |
| C2 | Contradiction recall (locked, default) | 0.250 | **0.500** | CONDITIONAL (improved; still < 0.90) |
| C3 | Contradiction recall (locked, opt-in cross-kind) | n/a | **1.000** | PASS (opt-in) |
| C4 | Zero project/version boundary violations | 0 | **0** | PASS |
| C5 | False-positive behaviour intact (Phase 1/2 matrices) | pass | **pass** (Phase 1 fp=0/fn=0; Phase 2 precision 1.000) | PASS |
| C6 | Cross-kind + negation covered by a dev set | no | **yes** (23 cases) | PASS |
| W1 | 8-client write p99 < 250 ms (Phase 2 harness) | 132 ms | **141–183 ms** | PASS |
| W2 | 16-client write p99 < 250 ms | 848 ms | **951–1,256 ms** | FAIL (root cause identified) |
| W3 | Zero concurrency errors / busy / stale | 0/0/0 | **0/0/0** | PASS |
| W4 | Root cause of the tail isolated | no | **yes** (per-op connection churn) | PASS |
| W5 | ≤8-client operational budget | yes | **retained** | PASS |
| N1 | Real neural retrieval run | BLOCKED | **BLOCKED** | BLOCKED |
| D1 | Reproduce 106/144 @ 3,840 vs 109/144 @ 5,674 | n/a | **exact** | PASS |
| D2 | Budgeted selector retains all top-five findings | no | **no** (loses 3 supporting) | FAIL → stay opt-in |
| D3 | Primary (gold) finding never lost; evidence preserved | untested | **gold 48/48, evidence loss 0** | PASS |
| V1 | gofmt / build / vet | pass | **pass** | PASS |
| V2 | Full learning package suite | pass | **pass** (57.8 s) | PASS |
| V3 | `go test -race` learning + hnsw | pass | **pass** (894 s / 3.3 s) | PASS |
| V4 | Repository-wide CLI suite | pass | **pass** (exit 0) | PASS |

## 2. What changed

One production change, in `cli/beacon/internal/learning/search_trust.go`:

- **Default path:** contradiction detection now also fires on (a) opposing action
  verbs (`stop`/`continue`, `keep`/`remove`, `enable`/`disable`, `allow`/`deny`,
  `accept`/`reject`, `include`/`exclude`) and (b) a negation asymmetry where exactly
  one side carries a negation word and at least one side is a directive. Both are
  gated by the existing scope, shared-term and overlap rules, so precision is unchanged.
- **Opt-in path:** new `TrustOptions.AllowCrossKind` (default `false`) lets a caller
  compare different `Kind` values. It is off by default because the Phase 1 labelled
  matrix deliberately treats a `Kind` difference as a scope boundary.

No other production code changed. New opt-in tests: `phase3b_contradiction_test.go`,
`phase3b_writer_test.go`, `phase3b_sync_test.go`, `phase3b_token_test.go`.

## 3. Blocker outcomes

1. **Contradiction (highest priority) — fixed + characterized.** Default recall on
   the observed locked set rose 0.25 → 0.50 (same-kind 3/6 → 6/6) at precision 1.000;
   the opt-in cross-kind mode reaches precision 1.000 / recall 1.000 on both the locked
   set and a new 23-case development set. The remaining default limitation (cross-kind)
   is explicit and opt-in. See `CONTRADICTION_ANALYSIS.md`.
2. **Sustained multi-writer reliability — characterized.** The 16-client write tail is
   **not** caused by index rebuilds (churn on/off identical), fsync (`synchronous`
   FULL/NORMAL/OFF differ by <20 %), or CPU (154–199 % of 800 %). It is caused by the
   **per-operation open/close of a fresh `sql.DB` pool** in every store call: a shared
   pool gives 16 concurrent writers p99 ≈ **214 µs** versus the store's ≈ **940 ms**.
   The ≤8-client budget is retained. See `WRITER_LATENCY_ANALYSIS.md`.
3. **Neural retrieval — BLOCKED.** The Mac is still unreachable and WSL2 still has no
   `torch`, `sentence-transformers`, or `BAAI/bge-small-en-v1.5` cache. Exact missing
   prerequisite recorded. See `NEURAL_RETRIEVAL_STATUS.md`.
4. **Token-budget evidence preservation — characterized + regression-tested.** The
   three lost findings are all supporting (gold 48/48 retained); no provenance is
   stripped from items that had it; evidence-preserving dedup retains the most findings
   (112/144). Budgeting stays opt-in. See `TOKEN_BUDGET_ANALYSIS.md`.

## 4. Defects and fixes

| Defect | Class | Fix | Evidence |
|---|---|---|---|
| Contradiction missed non-marker negations and antonym directives | inherited limitation | antonym-pair + negation-asymmetry signals (default) | `contradiction_dev_and_locked.txt` |
| Contradiction missed all cross-kind conflicts | inherited limitation | `TrustOptions.AllowCrossKind` opt-in | `contradiction_dev_and_locked.txt` |
| 16-client write tail latency | inherited, root-caused | not landed (lifecycle-coupled); documented | `writer_connection_materialised.txt`, `writer_synchronous.txt` |

No failing test was deleted, no assertion weakened, and no locked label was changed.

## 5. Validation and resources

| Check | Result | Time |
|---|---|---|
| `gofmt -l .` | clean | — |
| `go build ./...` / `go vet ./...` | pass | — |
| `go test ./internal/learning/` | pass | 57.8 s |
| `go test -race ./internal/learning/ ./internal/hnsw/` | pass | 894 s / 3.3 s |
| `go test ./...` (CLI module) | pass (exit 0) | 100 s |
| Writer soak 8/16 clients × 30 s | 0 errors/busy/stale; RSS ≈ 214 MB; CPU 154–163 % | 63 s |

Bounded; no 1M ANN run, no paid workload.

## 6. Recommended next step

1. **Phase 3C:** cache one `*sql.DB` per `Store` (with a `Close()` and a
   `schemaInitialized` rework) to remove the per-operation connection churn; validate
   against the crash-injection suite and re-measure 8/16-client p99 before changing the
   client budget.
2. **Phase 3B (deferred) / 3C:** run the real BGE-small neural arm on the Mac against
   the locked retrieval fixture, once the sidecar and model are reachable.
3. Keep neural, ANN, hybrid, token budgeting, and cross-kind contradiction detection
   **opt-in**.
4. The six-task paired agent evaluation remains a separate subsequent phase.
