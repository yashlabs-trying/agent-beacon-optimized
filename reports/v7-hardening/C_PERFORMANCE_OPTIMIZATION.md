# Beacon V7 hardening — Phase C: Retrieval and End-to-End Performance

Status: **Phase C partially implemented.** Allocation reductions landed and are
committed; the cold first-use FTS penalty (Phase A) is diagnosed but not yet
eliminated; wall-clock on this host is SQLite- and I/O-bound and noisy.

Branch: `codex/beacon-v7-hardening`
Parent: Phase B `e2408766`
Source changed: `cli/beacon/internal/learning/search_question.go`,
`cli/beacon/internal/learning/context_select.go`
New benchmark: `cli/beacon/internal/learning/phase_c_bench_test.go`

## 1. Measured baseline (warm, 1,000-memory store)

| Path | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `ListMemories` (Limit 8) | ~58,000 | 2,594 | 64 |
| `SearchQuestion` (CandidateK 50) | ~1,400,000–1,600,000 | 248,382 | 2,099 |

The comparison is not equal-budget: question-aware search deliberately considers
CandidateK=50 candidates and runs trust/supersession/injection over them, whereas
`ListMemories` returns 8 rows. The overhead is inherent to the intelligence layer,
not a defect.

## 2. Profiling (pprof, warm, 300 iterations)

- **CPU:** dominated by `modernc.org/sqlite` (`_sqlite3VdbeExec`, `_sqlite3Parser`,
  `_sqlite3GetToken`) — i.e. the lexical query itself — plus `runtime.futex` (GC /
  locking). The V7 selection logic is not the CPU bottleneck.
- **Allocations:** dominated by `SelectTrustContext` (59.6% cumulative), specifically
  `candidatePairs` (24.8% flat), `resolveSupersessions` (35.9% cumulative), and
  `significantTerms` (13.2%). The same candidate bodies were tokenized repeatedly.

## 3. Changes (smallest general fixes)

1. `search_question.go` — per-call caches in `questionIntent`:
   - `termCache`: `memoryTerms` is computed once per memory instead of in
     `direct`/`slotMatch`/`primaryMatch`/`linked`.
   - `defCache`: the uncertainty (`definitive`) scan runs once per memory instead of
     in both the lexical and semantic passes.
2. `context_select.go` — `candidatePairs`:
   - computes `significantTerms` once per memory;
   - uses a **direct pairwise scan for ≤64 candidates** (the retrieval case), avoiding
     the inverted-index maps; the indexed path is retained for large candidate sets
     (≥65, e.g. the Phase 4D scaling test). Both produce the identical pair set
     (shared ≥2 significant terms), guarded by `TestContract_IndexedEqualsNaiveDetector`
     and `TestPhase4DSelectorScaling`.
3. `context_select.go` — `resolveSupersessions`: `significantTerms` is precomputed once
   per memory; the pairwise loop previously recomputed it up to four times per pair.

## 4. Result

| Metric | Before | After | Change |
|---|---:|---:|---:|
| `SearchQuestion` B/op | 248,382 | **207,988** | −16% |
| `SearchQuestion` allocs/op | 2,099 | **2,007** | −4% |
| `SearchQuestion` ns/op | ~1.4–1.6 ms | ~1.4–3.5 ms* | *unreliable (see §5) |
| Correctness | — | all tests pass | unchanged |

*Wall-clock on this host is dominated by SQLite and OneDrive/WSL I/O and varied by
>2× between runs on unchanged code; it is not a trustworthy signal here. Allocation
counts are stable and are the reported metric.

## 5. Honest limitations

- **Cold first-use FTS penalty (Phase A: original 14.7 ms vs V7 43.7 ms) is not fixed
  in this pass.** The Phase A root cause — first-use FTS readiness — requires preparing
  derived state outside the latency-sensitive query or incrementally, which is a larger
  change; it remains open.
- Warm wall-clock was not reliably improved beyond allocation reductions; the dominant
  warm CPU cost is the SQLite lexical query, which is shared with original Beacon.
- The stretch targets (2× warm, 1.5× p95, 15% e2e p95, 20% cost/task) are **not met**
  and are not claimed.
- End-to-end agent latency was not re-measured in Phase C (no model run); it is
  **NOT TESTED** here.

## 6. Verification

- `TestSearchQuestion*` (12), `TestContract_*` (12), `TestPhase4DSelectorScaling` — pass.
- Full learning suite: **ok 58.0 s**. `internal/hnsw`: ok. `go vet`: clean. `go build ./...`: pass.
- Default path unchanged; `SearchQuestion` remains opt-in.

## 7. Commit

Committed on `codex/beacon-v7-hardening` (Phase C).
