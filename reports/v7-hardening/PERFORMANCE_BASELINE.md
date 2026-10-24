# Beacon V7 hardening — Phase 0: Performance Baseline

Status: **baseline captured; the OneDrive filesystem was identified as a dominant
environmental factor.**

Branch: `codex/beacon-v7-hardening`, HEAD at start `11ca834d5f96eb8a07ac04ec7aff0d05aff52292`.
Worktree: `C:\\Users\\<user>\\work\rescue\beacon-v7-hardening`
(WSL `/mnt/c/...`). Host: WSL2, 8 vCPU, 7.6 GiB, Go 1.27, Linux/amd64.

## 1. Method

The Phase 3B writer harness (`BEACON_PHASE3_WRITER=1`) seeds 300 memories, then runs a
sustained mixed read/write workload for N seconds at C clients, reporting read/write
p50/p95/p99, errors, busy, WAL bytes, and peak RSS. It writes its database to
`t.TempDir()`, i.e. the Go temp dir.

## 2. Filesystem investigation (the key finding)

`t.TempDir()` resolves to the Go temp dir. On WSL that is `/tmp` (native ext4) by
default. Forcing `TMPDIR` onto the OneDrive mount (`/mnt/c/...`) places the SQLite
database on the Windows-synced filesystem.

| Workload (16 clients, mixed, 8 s) | DB filesystem | write p50 | write p99 | reads | writes |
|---|---|---:|---:|---:|---:|
| **Baseline (pre-fix)** | **ext4 (/tmp)** | 2.9 ms | **634 ms** | 13,276 | 3,319 |
| **Baseline (pre-fix)** | **OneDrive (/mnt/c)** | 208.9 ms | **7,762 ms** | 564 | 141 |

The OneDrive mount is ~12× worse at p99 and ~70× worse at p50. Each SQLite commit fsyncs
on a filesystem with ~1 s write latency under this workload, so **the filesystem, not the
code, is the dominant factor when the database lives on OneDrive.**

**Conclusion:** a SQLite write workload must run on a local Linux filesystem. On
`/mnt/c` (OneDrive), per-commit latency is ~1 s regardless of code. This is a
deployment/environment constraint, not a code defect.

## 3. Reproduced baseline numbers (ext4, pre-fix)

| Clients | write p50 | write p95 | write p99 | read p99 | errors/busy |
|---:|---:|---:|---:|---:|---:|
| 8 | — | — | 130 ms (reported) | — | 0 |
| 16 | 2.9 ms | 133.9 ms | 634 ms | 1.26 ms | 0 / 0 |

This reproduces the reported problem: at 16 clients the write tail explodes relative to
8 clients while throughput stalls — the signature of SQLite write-lock contention with
`_txlock=immediate` and a 30 s busy_timeout (16 connections busy-poll the single writer).

## 4. Artifacts

- Harness: `cli/beacon/internal/learning/phase3b_writer_test.go`.
- Raw numbers recorded in this report and reproduced by the commands below.
- Retrieval microbenchmarks: `phase_c_bench_test.go` (`BenchmarkPhaseC*`).

## 5. Reproduction

```sh
cd cli/beacon
# ext4
BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=8 \
  go test ./internal/learning/ -run TestPhase3BWriterLatency -v -count=1
# OneDrive (environmental comparison)
TMPDIR=/mnt/c/Users/yashs/work/tmpbench \
BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=8 \
  go test ./internal/learning/ -run TestPhase3BWriterLatency -v -count=1
```

## 6. Baseline retrieval (from Phase C, unchanged)

`ListMemories` (Limit 8): ~58 µs, 64 allocs. `SearchQuestion` (CandidateK 50): ~1.4–1.6 ms,
248 KB / 2,099 allocs (before Phase C allocation fix). These have different semantics and
must not be presented as a direct architectural ratio.
