# Phase 1 — Concurrency (22 October 2026 WSL2 revalidation)

`BEACON_CONCURRENCY_MATRIX=1 go test -run '^TestConcurrencyPhase1Matrix$' -v -timeout 30m ./internal/learning` completed 20 cells: 1, 4, 8, 16 clients times read, write, mixed, supersede, and rebuild, with 25 operations per client (25–400/cell). All cells had **zero errors**. The test asserts project isolation and final write/supersession state. Measurements are from the WSL2 Ubuntu laptop, not the Mac.

| Clients | Read ops/s, p99 ms | Write ops/s, p99 ms | Mixed ops/s, p99 ms | Supersede ops/s, p99 ms | Rebuild ops/s, p99 ms |
|---:|---:|---:|---:|---:|---:|
| 1 | 3048, 0.51 | 1403, 1.44 | 910, 2.05 | 1286, 1.34 | 3262, 0.67 |
| 4 | 5075, 1.86 | 1036, 54.60 | 1192, 19.80 | 967, 54.68 | 5575, 2.02 |
| 8 | 5523, 2.82 | 1004, 105.03 | 1223, 80.95 | 1004, 129.96 | 5604, 2.30 |
| 16 | 4759, 6.21 | 703, 195.33 | 698, 232.63 | 417, 532.29 | 4511, 6.75 |

Peak process RSS in the 16-client cells was 24.0–26.4 MB. p95 values and each cell's exact output were printed by the test. The 16-client tail latency shows SQLite write serialization; the test is a short burst, not a soak or multi-process benchmark. `go test -race -count=10 -run '^TestSearchConcurrentReadersAndWriters$' ./internal/learning` passed on WSL2 after the busy-timeout fix. Full learning race suites passed on Mac in 750.333 s (`artifacts/mac_learning_race.txt`) and WSL2 (`artifacts/wsl_learning_race_result.txt`, exit code 0). Before the fix, a long WSL race run failed with `SQLITE_BUSY` and only 84/100 expected writes; a deterministic 11-second lock test reproduced the timeout and now passes. The earlier 643.8-second race pass below predates this revalidation and does not replace the new evidence.

## Prior Phase 1 report (preserved below)

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
