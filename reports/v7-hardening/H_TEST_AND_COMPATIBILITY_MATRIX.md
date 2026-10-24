# Beacon V7 hardening — Phase H: Test and Compatibility Matrix

Status: **Core suites pass; several opt-in/agent-level tests are NOT TESTED.**

## 1. Executed on the Phase C commit (this host, WSL2)

| Check | Command | Result |
|---|---|---|
| Learning unit/integration suite | `go test ./internal/learning/ -count=1` | **ok 58.0 s** |
| HNSW package | `go test ./internal/hnsw/ -count=1` | ok |
| Race detector (concurrency/crash subset) | `go test -race ./internal/learning/ -run 'Concurrency\|Concurrent\|PutMemoryWaits\|Crash'` | **ok 39.3 s** |
| `go vet` | `go vet ./internal/learning/` | clean |
| Build | `go build ./...` | pass |
| gofmt | `gofmt -l internal/learning` | clean |
| Concurrency stress (8 clients) | `BEACON_PHASE2_STRESS=1 go test -run TestPhase2ConcurrencyStress` | PASS (0 errors/busy/stale) |
| Writer latency (16 clients) | `BEACON_PHASE3_WRITER=1 go test -run TestPhase3BWriterLatency` | PASS (p99 measured; see Phase D) |
| V7 question tests | `go test -run TestSearchQuestion` | 12/12 PASS |
| Contract tests | `go test -run TestContract_` | 12/12 PASS |

## 2. Env-gated / not-run tests (not concealed)

| Test | Gate | Status |
|---|---|---|
| `TestPhase2ConcurrencyStress` | `BEACON_PHASE2_STRESS=1` | run in Phase D |
| `TestPhase3BWriterLatency` | `BEACON_PHASE3_WRITER=1` | run in Phase D |
| `TestPhase3ARetrievalSemantic` | `BEACON_PHASE3_RETRIEVAL=1` + embedder | **NOT RUN** (no sidecar in this pass) |
| ANN matrix tests | env-gated | **NOT RUN** |
| Phase 3D six-task agent harness | needs a model | **NOT RUN** (Phase 5 covered agent tasks separately) |

## 3. Repository-wide CLI suite

`go test ./...` across the whole CLI module was **NOT RUN to completion** in this pass
(it is large and slow on this OneDrive/WSL host). Only the `internal/learning` and
`internal/hnsw` packages plus `cmd` (memory tests, Phase 4D) were exercised. This is a
coverage gap, not a pass.

## 4. MCP integration

The previously excluded OpenCode MCP integration test was **NOT investigated** in this
pass. It is recorded here as an open item: determine whether the exclusion is an
environment/configuration issue or a real defect before release.

## 5. Cross-platform

Only Linux (WSL2) was tested. macOS/Windows behaviour: **NOT TESTED** in this pass.

## Verdict

Core correctness/race/vet/build: **PASS**. Full repository CLI suite, ANN/neural
harnesses, MCP integration, and cross-platform: **NOT TESTED** (gaps recorded).
