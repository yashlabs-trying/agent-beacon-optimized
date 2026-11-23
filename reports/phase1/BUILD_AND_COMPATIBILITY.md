# Phase 1 — Build and Compatibility (22 October 2026)

Recovered V6 base `85e6ea1cbb25ac193bd902966aa5889bf174dfa3`; untouched original baseline `99c709081ad49da1c5ade205929b61b19360ed9b`. The complete unpublished history was preserved in `../../../upstream-history.bundle` (outside this repository), then cloned into isolated WSL2 (`/home/<user>/work/phase1-v6`) and Mac (`~/beacon-phase1-v6`) workspaces. Both HEADs matched the base SHA before edits. The original Windows checkout remained clean. Passwordless authenticated SSH to the Mac worked. No remote Git operation was used to publish work. Exact OS/tool versions and point-in-time resources are in `artifacts/host_manifest.txt`.

| Host | Environment | Go build/vet | Tests |
|---|---|---|---|
| Windows laptop WSL2 | Ubuntu, 8 vCPU, about 8 GiB RAM, Go 1.27.3 | `go build ./...`, `go vet ./...` pass | `go test -count=1 ./internal/learning` passes (82.6 s in full suite); full `go test -race -timeout 30m -count=1 ./internal/learning` passes. Repository-wide `go test -p 2 -timeout 30m -count=1 ./...` has one OpenCode E2E failure described below. |
| Apple M5 Mac | macOS arm64, 16 GiB unified memory, Go 1.27.3, neural venv Python 3.12 | `go build ./...`, `go vet ./...` pass | `go test -p 2 -timeout 30m -count=1 ./...` passes; `go test -race -timeout 30m -count=1 ./internal/learning` passes (750.333 s). |

Run commands from `cli/beacon` after `make build-hooks-current` (required generated embedded hook asset). Changed Go files produced no output from `gofmt -l`. `python -m unittest cli/beacon/internal/learning/test_embed_sidecar.py` passed in the canonical checkout. Raw Mac logs are `artifacts/mac_full_test.txt` and `artifacts/mac_learning_race.txt`; the WSL full-race exit record is `artifacts/wsl_learning_race_result.txt`.

On WSL2, `TestMCPConnectEndToEndOpenCode` fails because the installed `opencode mcp list` reports “No MCP servers configured” after the test expects the Beacon server. The same exact test on a detached, **untouched** `99c70908` worktree fails with the same output under the same PATH. With a minimal PATH lacking OpenCode, it skips. This is an existing environment/integration failure outside the changed learning package; it is recorded rather than hidden. On Mac, an initial full-suite run under concurrent model download failed six 2-second fake-executable discovery tests, but targeted reruns passed and the later `-p 2` full suite passed.

The V6 default retrieval path still needs no GPU, model, sidecar, or network; neural and ANN paths remain opt-in. This report describes tested source behavior, not a binary compatibility guarantee for every CLI version. The earlier WSL-only record, including its then-valid Mac access blocker, is preserved below.

## Prior Phase 1 report (preserved below)

# Phase 1 — Build and Compatibility

## Commands

```bash
cd cli/beacon
gofmt -l internal/learning internal/hnsw      # (LF-normalized)
go build ./...
go vet ./internal/learning/...
go test -count=1 ./internal/learning/...
go test -race -count=1 ./internal/learning/...
```

## Results (WSL2, revision `aee750a5`)

- gofmt: clean for all new/modified files (`internal/learning`, `internal/hnsw`).
- `go build ./...`: OK.
- `go vet ./internal/learning/...`: OK.
- Full `go test`: **ok** (79.7 s).
- `go test -race`: **ok** (643.8 s).

## Compatibility

- Public API unchanged: `Open`, `Query`, `ListMemories` keep their signatures and
  semantics. Neural (`OpenSemantic`, `HTTPEmbedder`) and ANN (`UseANN`) are opt-in.
- New dependency surface: vendored `internal/hnsw` (MIT) replaces the upstream
  `github.com/coder/hnsw`; transitive `golang.org/x/exp/maps`, `viterin/vek` remain.
- Default path requires no GPU, model, sidecar, or network.

## Pre-existing vs new failures

The `learning` package passes fully. Repository-wide pre-existing failures
(hooks/connect env tests) are unchanged from baseline and are not attributable to
this work.

## Notes

- macOS build validation: **NOT TESTED** (Mac unreachable). The code is pure Go +
  modernc.org/sqlite (no cgo), so macOS is expected to build, but this was not
  executed.
