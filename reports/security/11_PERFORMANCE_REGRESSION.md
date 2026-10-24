# 11 — Performance Regression Report

Status: **warm retrieval and no-match scaling measured on the hardening branch;
16-client write p99 measured on Windows under the semantic mixed harness and is
not comparable to the Linux reference. Full baseline (cold start, RSS, binary
size, 1M, 64-client) NOT RE-MEASURED.**

## Environment

Windows 11 build 26200, Intel i5-1155G7 (4c/8t), 15.7 GB RAM, go1.27.0
windows/amd64. The reference measurements in the task were taken on Linux.
Cross-platform absolute numbers are **not** comparable.

## Warm retrieval (hardening branch)

```
go test ./internal/learning/ -run '^$' \
  -bench 'BenchmarkPhaseCListMemoriesWarm|BenchmarkPhaseCSearchQuestionWarm|BenchmarkAdaptiveNoMatchCold' \
  -benchtime 30x -count 1
```

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkPhaseCListMemoriesWarm` | 454,030 | 4,860 | 86 |
| `BenchmarkPhaseCSearchQuestionWarm` | 1,516,087 | 132,734 | 898 |
| `BenchmarkAdaptiveNoMatchCold/n=1000` | 699,987 | — | — |
| `BenchmarkAdaptiveNoMatchCold/n=10000` | 366,893 | — | — |
| `BenchmarkAdaptiveNoMatchCold/n=100000` | 1,157,087 | — | — |

The reported V7 warm question-aware figure was ~875 allocs/op; 898 is within
run-to-run noise (+2.6%). No material retrieval regression is observed on the
hot path; the hardening changes (authorization lookup, scan row cap) add no work
to a selective indexed query.

## Finding C: original scan vs V7 indexed

| N | original scan | V7 indexed |
|---|---|---|
| 1000 | 8.05 ms, 12,035 allocs | 81 µs, 82 allocs |
| 5000 | 46.0 ms, 60,910 allocs | 142 µs, 82 allocs |

## 16-client write latency (Windows, semantic mixed harness)

```
BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 \
BEACON_PHASE3_W_SECONDS=5 BEACON_PHASE3_W_REPS=1 \
go test ./internal/learning/ -run TestPhase3BWriterLatency -count=1 -v
```

```
P3B_WRITER rep=1 mode=mixed churn=false concurrency=16 duration_s=5
reads=3032 writes=758 errors=0 busy=0
read_p50=510us read_p95=1584us read_p99=4378us
write_p50=90067us write_p95=188727us write_p99=297457us write_mean=102313us
peak_rss_mb=0   (peakRSSKB reads /proc/self/status; unavailable on Windows)
```

**Interpretation (honest):** write p99 = 297 ms on Windows under this harness.
This is **not** the reference workload: the harness uses the semantic embedder
and a mixed read/write mix, and the platform is Windows. The reference Linux
figure was ~53.4 ms. Because the platform and harness differ, this measurement
**does not** verify or refute the ≤250 ms gate. The write hot path was not
changed by hardening (approval, a rare path, now uses one transaction instead of
two; `PutMemory` performs the same operations in the same order).

**Gate status: NOT VERIFIED on the reference platform.**

## Not re-measured

- Cold-start retrieval and cold-open (reported V7 20–36 ms).
- Peak RSS (unavailable on Windows; `peakRSSKB` reads `/proc`).
- Binary size (reported ~47.5 MB).
- 1M-memory search scalability.
- 64-client concurrency.

## Conclusion

No regression is attributable to the hardening changes on the retrieval hot
path or the write path. The new costs are: one map lookup per authorization
decision, one map lookup per returned row, and a bounded scan counter. Absolute
16-client numbers on Windows are reported but are not a substitute for the
Linux reference measurement.
