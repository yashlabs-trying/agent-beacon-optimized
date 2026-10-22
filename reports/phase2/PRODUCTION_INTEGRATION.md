# Phase 2 — Production Integration

## Summary

The V6 memory engine still compiles and its public consumers still pass. The
storage layer is idempotent, upgrades cleanly from the original (pre-V6) Beacon
schema, rebuilds derived indexes after deletion, reopens after an interrupted
startup, and isolates embeddings by model version. Default CLI/MCP/dashboard
paths are lexical-only, so offline operation and the no-neural-model fallback are
preserved by construction.

## Environment

Apple M5 / macOS 26.6.2 / go1.27.3. Worktree base `85e6ea1c`.

## Build and consumer compatibility

| Check | Command | Result |
|---|---|---|
| Full CLI build | `go build ./...` (from `cli/beacon`) | PASS |
| MCP server tests | `go test ./internal/mcpserver/` | PASS |
| Dashboard memory tests | `go test ./internal/endpoint/dashboard/` | PASS |
| CLI memory command tests | `go test ./cmd/ -run 'Memory\|Learning'` | PASS |

Consumers of `internal/learning`: `internal/mcpserver/server.go`,
`internal/endpoint/dashboard/memory.go`, `cmd/memory.go`. All use
`learning.Open` (lexical-only). `OpenSemantic` and `HTTPEmbedder` are opt-in and
are not wired into any shipping command, so there is no runtime neural-service
dependency to fail: the default is offline lexical operation. (This is a
macOS build/test run; no Linux-only build was performed in this session.)

## Lifecycle tests (`phase2_integration_test.go`)

| Test | What it verifies | Result |
|---|---|---|
| `TestPhase2IdempotentInit` | opening the same store three times is safe and searchable | PASS |
| `TestPhase2UpgradeFromOriginalStorage` | a database containing only the original `evaluations`/`candidates`/`memories` tables (no derived schema) opens and searches; the derived schema is created and the index built lazily | PASS |
| `TestPhase2IndexRebuildAfterDeletion` | dropping `memories_fts` and clearing `memory_index_meta` triggers a rebuild on the next search | PASS |
| `TestPhase2ReopenAfterInterruptedStartup` | dropping all derived tables and meta, then reopening, recovers | PASS |
| `TestPhase2ANNLifecycleAcrossGenerations` | a write advances the generation, invalidates the ANN cache, and the rebuilt graph contains the new node | PASS |
| `TestPhase2EmbeddingModelVersionIsolation` | embeddings are stored with the model id; opening with a different model re-embeds on rebuild and old vectors are not used | PASS |
| `TestPhase2LargeMemory` | a ~1 MB body round-trips and is searchable | PASS |
| `TestPhase2CorruptRowSurfacedNotSilent` | a hand-corrupted `memory_json` row surfaces as an error rather than a panic or a silent drop | PASS |

## Schema and migration

- Authoritative schema is versioned by `PRAGMA user_version`; the derived index
  has an independent `index_version`. A layout change marks the index not-ready
  and rebuilds lazily, so it never requires migrating `memories`.
- Initialisation is idempotent (`CREATE ... IF NOT EXISTS`) and the derived
  schema is best-effort: if it cannot be created, search falls back to the
  original scan.

## Notes / limitations

- macOS build/test only; no WSL2/Linux clean-build run in this session.
- The corrupt-row behaviour is "surface an error", which is deliberate
  (data-loss-visible) but means one bad row can fail a search; this was not
  changed.
- The reproducibility script `reports/phase2/run_phase2_benchmarks.sh` is
  portable to macOS and Linux and writes raw output under `reports/phase2/raw/`.
