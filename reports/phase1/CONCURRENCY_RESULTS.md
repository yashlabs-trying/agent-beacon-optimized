# Phase 1 — Concurrency

## Verified (WSL2)

- `go test -race ./internal/learning/...` → **PASS** (643.8 s).
- `TestSearchConcurrentReadersAndWriters` (8 readers, 4 writers): no race, no
  cross-project leak, all acknowledged writes visible.
- `TestSearchConcurrentSupersedeAndRead` (6 readers, 3 superseding writers): no
  superseded memory returned, no torn state.
- `TestSearchCrossStoreVisibility`: cross-store write visible on the next query.
- SQLite busy: `_txlock=immediate` + `_busy_timeout`; no `SQLITE_BUSY` write loss
  observed under the tested load.

## Measured

- Race suite wall time: 643.8 s (8 vCPU, 8 GB). Correctness-focused, not a
  latency benchmark.

## NOT TESTED

- 16 / 32 / 100 / 500 concurrent clients (Phase 1 kept to the 8-reader/4-writer
  and 6/3 harnesses to avoid overwhelming the 8 GB box).
- Sustained soak, p95/p99 under load, goroutine/FD leak soak, write starvation.
- Multiple independent OS processes (tested in-process + cross-connection only).

## Pass condition

Race and concurrency tests pass for the tested configurations: **PASS**.
Higher-client-count and soak behavior remain unverified.
