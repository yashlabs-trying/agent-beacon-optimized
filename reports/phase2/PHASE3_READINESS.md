# Phase 2 — Phase 3 Readiness

## Verdict

The Phase 2 work is complete and committed on
`test/beacon-v6-phase2-opencode` (base `85e6ea1c`). The memory engine is
correct and durable in the tested envelopes, HNSW construction is 3.5–5.6×
faster with recall intact, and the trust/security boundary held under
adversarial input. **Phase 3 can proceed**, with the caveats below.

## Ready for Phase 3

- **HNSW construction** is no longer a systems risk at the 100K scale (68 s,
  387 MB, Recall@10 0.9949).
- **Durability** of acknowledged writes under process crash is demonstrated
  across every write stage.
- **Scope and approval** boundaries held under adversarial testing.
- **Default lexical/offline path** is unchanged and model-free.

## Blockers and pre-Phase-3 work

| Priority | Item | Why it matters | Suggested action |
|---|---|---|---|
| High | Neural-embedding end-to-end validation | All ranking/semantic numbers here are **hashed-semantic**, not neural. Codex owns the Mac neural sidecar; the two must be reconciled before any neural default. | Run the same held-out ranking/token eval with the real BGE-small sidecar; compare. |
| High | Concurrency under sustained multi-writer load | Write p99 reaches ~0.75 s at 8 writers, ~1.3 s at 32; frequent index rebuilds block readers. | Decide an operational writer budget, or add a pooled connection and/or a non-blocking rebuild strategy. |
| Medium | Low-`M` recall | `M=16`/`M=32` cannot meet the 0.95 target on this data; `M=64` is the only safe default, which is the expensive one. | Implement the canonical diversity neighbour-selection heuristic (Malkov–Yashunin Alg. 4) and re-sweep `M`. |
| Medium | Contradiction recall/cross-talk | Recall 0.758 (cross-kind misses); 37 cross-talk pairs on a dense corpus. | Extend detection across kinds and/or raise the overlap threshold; validate on held-out data. |
| Medium | Token-budget evidence loss | Budgeting loses ~6% of distinct findings. | Adopt evidence-preserving dedup first; make budgeting opt-in with a measured tradeoff. |
| Low | Ranking default | RRF is below semantic-only on held-out data. | Enable a semantic-heavy weight only after real-embedding held-out evidence. |
| Low | Build determinism | Build is not bit-reproducible. | Seed the RNG by default if reproducible builds are required. |

## Cross-workstream reconciliation with Codex (Phase 1)

- **Duplicate-fix risk:** Codex has uncommitted Phase 1 test files
  (`ann_phase1_matrix_test.go`, `crash_recovery_test.go`, `trust_phase1_matrix_test.go`,
  `concurrency_phase1_matrix_test.go`, `security_smoke_test.go`, `busy_lock_test.go`)
  plus edits to `store.go`, `search_ann.go`, `search_context.go`, `search_trust.go`,
  `embed_sidecar.py`. These were **not** used or cherry-picked. They overlap in
  subject with the Phase 2 suites and should be merged by a human reviewer, not
  automatically.
- **Likely conflicts:** `search_ann.go` (Codex added `BEACON_ANN_M` and stage
  timing; Phase 2 added `EfConstruction`/`Seed`), `search_trust.go` (Phase 2
  changed `detectContradictions`), `store.go` (Phase 2 added `faultHook`).
- **Shared regression coverage to keep:** the Phase 2 HNSW invariant tests, the
  crash-injection harness, the adversarial trust matrix, and the held-out
  ranking/token evaluation are good candidates for shared coverage.
- **Platform note:** Phase 1's HNSW numbers were on slower/different hardware;
  Phase 2's are Apple M5. Compare only matched runs.

## Not tested (explicitly)

- 1M HNSW (out of scope; memory).
- Power-loss durability (process crash only).
- Real neural inference and end-to-end agent-task success.
- WSL2/Linux execution.
- A 1-hour soak (a 15-minute soak was run).

## Reproduction

`reports/phase2/run_phase2_benchmarks.sh` (portable macOS/Linux) regenerates all
Phase 2 raw output under `reports/phase2/raw/`. Every report lists its exact
command.
