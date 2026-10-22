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
