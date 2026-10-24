# Raw Evidence Manifest

Every entry records the exact commit, environment, command, fixture, and result.
Environment: Windows 11 build 26200, Intel i5-1155G7 (4c/8t), 15.7 GB RAM,
go1.27.0 windows/amd64, git 2.55.0.windows.3. Authoritative upstream platform is
Linux; Windows-specific limitations are noted per entry.

## Commits

| Label | SHA |
|---|---|
| Frozen original | `99c709081ad49da1c5ade205929b61b19360ed9b` |
| Earlier stabilized V7 | `0cdbeb6af8b7786e302231cc18c98a25260637da` |
| V7 baseline (start HEAD) | `7c17c5fd1503769e0fc147c0f9d3c551ee89c539` |
| Baseline tags | `baseline-v7-7c17c5fd`, `baseline-original-99c70908` |
| Hardening branch | `codex/beacon-v7-security-hardening` |

## E-01 — Baseline build

- Command: `go build ./...` (workdir `cli/beacon`)
- Result: exit 0.
- Limitation: two Unix-only test files blocked the Windows test build; given
  `//go:build !windows` and their neutral helpers moved to
  `crash_helpers_test.go`. No production code affected.

## E-02 — Cross-commit source identity (Findings A, B)

- Commands:
  - `git diff --stat 99c70908 7c17c5fd -- cli/beacon/internal/mcpserver/server.go` → empty
  - `git diff --stat 99c70908 7c17c5fd -- cli/beacon/internal/learning/candidate.go` → empty
  - `git diff --stat 99c70908 7c17c5fd -- cli/beacon/internal/learning/store.go` → 228 insertions, 48 deletions
- Conclusion: Findings A and B reproduce identically on the frozen original and
  the V7 baseline; Finding C is the `store.go` search change.

## E-03 — Finding A reproduction (V7 baseline, default mode)

- Command: `go test ./internal/mcpserver/ -run '^TestBaseline' -count=1 -v`
- Fixture: synthetic project `project-secret`, memory `memory-secret`
  (`security_baseline_repro_test.go`).
- Result: PASS, with logged confirmation:
  - `get_memory` returned `{"id":"memory-secret",...,"body":"Internal credential rotation procedure.","project":{"id":"project-secret"}}`
  - `search_memory` returned the memory for a caller-supplied `project_id`.
- Interpretation: cross-project disclosure in the unscoped ID path and the
  caller-controlled search scope.

## E-04 — Finding C benchmark (original scan vs V7 indexed)

- Command: `go test -c -o <tmp>/learning.test.exe ./internal/learning/` then
  `learning.test.exe -test.bench=BenchmarkFindingCSearchNoMatch -test.benchtime=20x`
- Fixture: `security_finding_c_test.go`, `proj` with N synthetic memories, no-match query.
- Result:

| Sub-benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| original-scan/N=1000 | 8,050,425 | 1,394,642 | 12,035 |
| v7-indexed/N=1000 | 81,315 | 4,024 | 82 |
| original-scan/N=5000 | 46,036,835 | 7,241,691 | 60,910 |
| v7-indexed/N=5000 | 142,215 | 4,024 | 82 |

## E-05 — Authorization enforcement (Phase 2)

- Command: `go test ./internal/mcpserver/ -count=1`
- Result: `ok ... internal/mcpserver` (includes all `TestAuthorization*` and the
  baseline repros).
- Tests: see report 03.

## E-06 — Atomic lifecycle (Phase 4)

- Command: `go test ./internal/learning/ -run 'TestApprovalCrashAtomicity|TestConcurrentApprovalSingleWinner|TestRejectedCandidateCannotBeApproved' -count=1 -v`
- Result: all PASS. Crash stages: `tx_begin_pre`, `tx_begin_post`,
  `memory_insert`, `fts_update`, `generation_update`, `pre_commit`,
  `post_commit`.

## E-07 — Full learning suite (Windows)

- Command: `go test ./internal/learning/ -count=1`
- Result: exit 1, but **every** failure is `TempDir RemoveAll cleanup: unlinkat
  ... memory.db: The process cannot access the file because it is being used by
  another process` — a Windows harness artifact (tests leave SQLite handles open;
  Linux unlinks open files). No assertion failures were observed. Log:
  `%TEMP%\opencode\learning_full.log`.

## E-08 — Vet

- Command: `go vet ./internal/mcpserver/ ./internal/learning/ ./cmd/`
- Result: exit 0.

## E-09 — Warm retrieval + no-match scaling (hardening branch)

- Command: `go test ./internal/learning/ -run '^$' -bench 'BenchmarkPhaseCListMemoriesWarm|BenchmarkPhaseCSearchQuestionWarm|BenchmarkAdaptiveNoMatchCold' -benchtime 30x -count 1`
- Result:
  - `BenchmarkPhaseCListMemoriesWarm` 454,030 ns/op, 4,860 B/op, 86 allocs/op
  - `BenchmarkPhaseCSearchQuestionWarm` 1,516,087 ns/op, 132,734 B/op, 898 allocs/op
  - `BenchmarkAdaptiveNoMatchCold/n=1000` 699,987 ns/op
  - `BenchmarkAdaptiveNoMatchCold/n=10000` 366,893 ns/op
  - `BenchmarkAdaptiveNoMatchCold/n=100000` 1,157,087 ns/op

## E-10 — 16-client writer latency (Windows, semantic mixed harness)

- Command: `BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=5 BEACON_PHASE3_W_REPS=1 go test ./internal/learning/ -run TestPhase3BWriterLatency -count=1 -v`
- Result: `write_p99=297457us write_p50=90067us write_mean=102313us reads=3032 writes=758 errors=0 busy=0 peak_rss_mb=0`
- Limitation: Windows + semantic mixed harness; not comparable to the Linux
  reference (~53.4 ms). Gate NOT VERIFIED.

## E-11 — Resource limits

- Commands: `go test ./internal/mcpserver/ -run TestResource -count=1`;
  `go test ./internal/learning/ -run 'TestSearchScanCapReturnsExplicitError|TestSelectiveQueryStaysUnderCap' -count=1`
- Result: all PASS.

## E-12 — Provenance adversarial suite

- Command: `go test ./internal/learning/ -run 'TestProvenance|TestTrust' -count=1 -v`
- Result: 7 provenance tests PASS; only `TestTrustNoCrossScopeLeakage` fails on
  the Windows TempDir cleanup artifact.

## Environment limitations (must be reported, not hidden)

1. Windows Application Control intermittently blocks freshly built test
   executables. `go test ./pkg/ -count=1` works; `go test -c` then running the
   binary is intermittently blocked. Blocked commands are marked NOT TESTED.
2. Windows `t.TempDir` cleanup fails for tests that leave a SQLite handle open.
   Not an assertion failure.
3. `race`, multi-process SQLite, 1M-memory scalability, and model-backed
   injection evaluation were **not** executed in this pass.
4. The independent 500-task intelligence holdout was not accessed.
