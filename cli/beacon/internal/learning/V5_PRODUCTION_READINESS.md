# V5 production readiness

## Completed and verified

- ANN recall fixed by vendoring + correcting `internal/hnsw`; recall@10 0.975 on
  structured vectors (≥0.95 target), 72× faster than exact at 100K.
- Corrected ANN wired into hybrid search as an opt-in path (`HybridOptions.UseANN`).
- Upstream `github.com/coder/hnsw` dependency removed; `go.mod` tidied.
- Existing V2 lexical engine, cache, fallback, trust, and token budgeting
  preserved. `go build`, `go vet`, full tests, and `-race` all pass.

## Missing validations

- ANN at 1M (in-memory M=64 is memory-bound; needs disk-backed/quantized index).
- Neural embeddings (no offline runtime/model integrated).
- Agent task success and end-to-end latency (no agent runtime).
- ANN update/delete cost, cold-start, and persistence/reload.
- Crash-during-write and soak testing.

## Dependency risks

- Vendored `internal/hnsw` is a fork of an MIT library; it must be kept in sync
  with upstream fixes, or upstreamed. Its transitive deps (`x/exp/maps`,
  `viterin/vek`) remain.
- No cgo, no native library, no hosted service is required by the default path.

## OS portability

- Pure Go throughout (vendored HNSW + modernc.org/sqlite). Builds on
  Windows/Linux/macOS, x86_64; Apple Silicon untested here.

## Operational costs

- Disk: ~2.1× for the trigram index.
- Writes: 373 µs → 608 µs per write (index maintenance).
- ANN memory: M=64 stores ~64 edges/node as Go maps; large in RAM at scale.
- Exact semantic search: 513 ms @100K — must be gated by store size.

## Security and reliability risks

- Cross-scope leakage: none measured (ANN index is per-project; hydration filters
  supersession). Must re-verify for a disk-backed ANN.
- Model download: none occurs; no silent third-party binary execution.
- ANN recall is validated on synthetic structured vectors, not on a real neural
  embedding space (none available). Recall on real embeddings is unproven.

## Remaining production blockers

1. A validated ANN at 1M (recall ≥0.95, bounded RAM, persistent).
2. A license-cleared, offline neural embedding backend.
3. End-to-end agent evaluation showing task-success improvement.
4. Crash/soak hardening and ANN lifecycle (update/delete/reload).
5. A held-out tuning set large enough to select a fusion weighting safely.

## Verdict

The ANN recall problem (the V5 mission's first task) is **solved and verified**.
Neural embeddings and agent evaluation are **not** delivered; both are blocked by
the environment and are reported as such. Do not ship the ANN path as the default
until 1M scaling and real-embedding recall are measured.
