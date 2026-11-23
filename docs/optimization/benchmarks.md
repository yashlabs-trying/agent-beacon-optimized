# Benchmarks and reproduction

How to reproduce the performance claims in [performance](performance.md). Every claim
maps to a source report under `reports/` and a committed benchmark or test.

## Environment

| Item | Value |
|---|---|
| Go | 1.27.0 (`CGO_ENABLED=0`) |
| Module | `cli/beacon` (run Go commands from inside it) |
| Reference host | WSL2 Ubuntu, 8 vCPU, 7.6 GiB RAM, Linux/amd64 |
| Database filesystem | **local** (ext4). Do **not** benchmark on `/mnt/c`/OneDrive or other synchronized/network mounts. |

Sanity check the toolchain and build:

```sh
cd cli/beacon
go build ./...
go vet ./internal/learning/
```

## 1. Write concurrency (the headline result)

The writer harness seeds 300 memories, then runs a sustained mixed read/write workload
for `N` seconds at `C` clients, reporting p50/p95/p99, errors, busy, WAL bytes, and peak
RSS. It writes its database into the Go temp dir.

```sh
cd cli/beacon

# 16 clients, 8 seconds, local ext4
BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=8 \
  go test ./internal/learning/ -run TestPhase3BWriterLatency -v -count=1

# concurrency stress (8 clients), invariant checks
BEACON_PHASE2_STRESS=1 go test ./internal/learning/ -run TestPhase2ConcurrencyStress -v -count=1
```

Expected (local ext4): 16-client write p99 well under 250 ms, **0 errors / 0 busy**.

**Filesystem control (shows the environmental factor, not a code change):**

```sh
TMPDIR=/mnt/c/Users/<you>/tmpbench \
  BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=8 \
  go test ./internal/learning/ -run TestPhase3BWriterLatency -v -count=1
```

This reproduces the ~1 s/commit ceiling of synchronized mounts. Use it to confirm the
effect is environmental; do not report it as optimization performance.

**Harness:** `cli/beacon/internal/learning/phase3b_writer_test.go`.
**Source report:** [`reports/v7-hardening/D_DATABASE_CONCURRENCY_FIX.md`](../../reports/v7-hardening/D_DATABASE_CONCURRENCY_FIX.md),
[`PERFORMANCE_BASELINE.md`](../../reports/v7-hardening/PERFORMANCE_BASELINE.md).

## 2. Retrieval allocations

```sh
cd cli/beacon
go test ./internal/learning/ -run '^$' -bench 'BenchmarkPhaseCSearchQuestionWarm' -benchmem -count=5
```

Expected (warm, 1,000-memory store, CandidateK 50): ~875 allocs/op, ~130 KB/op.
The companion `ListMemories` (Limit 8) control is ~58 µs / 64 allocs.

**Benchmark:** `cli/beacon/internal/learning/phase_c_bench_test.go`.
**Source report:** [`reports/v7-hardening/RETRIEVAL_ALLOCATION_FIX.md`](../../reports/v7-hardening/RETRIEVAL_ALLOCATION_FIX.md).

> Allocation counts are stable on this host; `ns/op` varies by >2× run-to-run because
> the warm path is SQLite/I/O bound. Report allocations, not wall-clock.

## 3. Indexed search vs full scan (scaling)

The security pass measured indexed no-match search staying flat in corpus size versus
an O(N) scan in the original. This is exercised by the bounded-search tests and the
scaling test.

```sh
cd cli/beacon
go test ./internal/learning/ -run 'TestPhase4DSelectorScaling|TestContract_IndexedEqualsNaiveDetector' -v -count=1
```

**Source report:** [`reports/security/00_EXECUTIVE_SUMMARY.md`](../../reports/security/00_EXECUTIVE_SUMMARY.md) (question 8),
[`07_RESOURCE_EXHAUSTION.md`](../../reports/security/07_RESOURCE_EXHAUSTION.md).

## 4. Database placement + migration

```sh
cd cli/beacon
go test ./internal/learning/ -run 'TestDatabasePathHonorsMemoryDirEnv|TestCheckPlacement|TestMigrateDatabase' -v -count=1
```

**Source report:** [`reports/v7-hardening/DATABASE_PLACEMENT_FIX.md`](../../reports/v7-hardening/DATABASE_PLACEMENT_FIX.md).

## 5. Full correctness suite

```sh
cd cli/beacon
go test ./internal/learning/ -count=1          # ~38–58 s on the reference host
go test -race ./internal/learning/ -run 'Concurrency|Concurrent|PutMemoryWaits|Crash' -count=1
go test ./internal/hnsw/ -count=1
```

Expected: learning suite ok; race subset ok; `go vet` clean.

---

## Baseline vs optimized: fair comparison

The comparators are frozen commits. To compare, check out the commit you want and run
the same harness; the writer harness DDL, fixtures, and workload are shared.

| Arm | Commit | Role |
|---|---|---|
| Original | `99c709081ad49da1c5ade205929b61b19360ed9b` | frozen baseline |
| V6 integration | `209a987bc96c29b5e5d811a794d0357f20214463` | frozen |
| Phase 5 | `02e34f98dec1c5d34a2f0d52904969edc39e389f` | frozen eval |
| V7 dev | `7b605aa84f0f570ba265fbda17fe6d99e927b10e` | optimization base |
| Optimized | `db341fbc1f32d16777ce21484163a6be71ecb688` | this work |

Guidance for a fair run:

1. **Same host, same filesystem, same load.** The dominant error source is the
   database placing on a synchronized mount.
2. **Same harness revision.** Use the writer/bench harness committed with the arm you
   compare, or pin one harness and point both arms at it.
3. **Report allocations and percentiles, not single wall-clock means.**
4. **Do not compare `ListMemories` and `SearchQuestion` as a ratio** — different
   semantics and candidate budgets.
5. **Label historical data.** Results without a recreatable seed/host are historical.

## Measurement definitions

| Metric | Definition | Instrument |
|---|---|---|
| write p99 | 99th-percentile per-write latency under N clients | writer harness |
| allocs/op | heap allocations per retrieval call | `go test -benchmem` |
| bytes/op | bytes allocated per retrieval call | `go test -benchmem` |
| no-match search | latency of a query with no selective terms, vs corpus size | scaling test |
| peak RSS | resident set during the writer probe | writer harness |

## What is out of scope

End-to-end **agent-task** success, model cost, and semantic-vs-lexical quality are
**not** benchmarked here and are **not** claimed. See [limitations](limitations.md).
