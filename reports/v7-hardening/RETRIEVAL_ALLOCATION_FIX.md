# Beacon V7 — Phase 2: Retrieval Allocation Optimization

Status: **PASS — 56.4% fewer allocations on the reference question-aware workload.**
No correctness or trust regression.

Files: `cli/beacon/internal/learning/context_select.go` (+ `phase_c_bench_test.go`).

## 1. Profile (warm `SearchQuestion`, 1,000-memory store, CandidateK 50)

`go tool pprof -alloc_objects` on `BenchmarkPhaseCSearchQuestionWarm` before the change:

| Allocator | Share of objects |
|---|---:|
| `intersectTerms` | 20.7% |
| `internal/bytealg.MakeNoZero` | 16.6% |
| `significantTerms` (cum) | 10.6% |
| `strings.FieldsFunc` | 5.8% |
| `resolveSupersessions` / `candidatePairs` | large cumulative |

Root causes found by profiling (not guessed):
1. **`significantTerms` was computed three times per call** — once in `candidatePairs`,
   again in `resolveSupersessions`, again in `detectContradictionsIndexed`.
2. **`intersectTerms` allocated a slice per candidate pair** even when only the count
   was needed.
3. **Supersession tokenized every candidate even when no memory could supersede**
   (no correction markers, no `SupersededBy`).

## 2. Changes (each focused, general)

1. **Compute terms once.** Added `significantTermsPerMemory`; `candidatePairs`,
   `resolveSupersessions`, and `detectContradictionsIndexed` now have `…WithTerms`
   variants that take the precomputed term sets. `SelectTrustContext` computes the term
   sets **once** and shares them across supersession and contradiction detection.
   Public wrappers keep the old signatures for existing callers/tests.
2. **No-alloc shared count.** Added `sharedCount(a, b)` and use it wherever only the
   number of shared terms matters (supersession gates). `intersectTerms` (which builds a
   slice) is now called only when a contradiction is actually confirmed.
3. **Correction-candidate guard.** Added `hasCorrectionCandidate`; when no memory has a
   `SupersededBy` link or a correction marker, the O(n) tokenization for supersession is
   skipped entirely (the common case for a plain fact store).

Correctness preserved: the pair set, the supersession decision, and the contradiction
decision are unchanged (guarded by `TestContract_IndexedEqualsNaiveDetector`,
`TestContract_*`, `TestSelectContext_*`, `TestPhase4DSelectorScaling`,
`TestPhase4CMemoryHistory`).

## 3. Result (reference warm workload)

| Metric | Before | After | Change |
|---|---:|---:|---:|
| **Allocs/op** | 2,007 | **875** | **−56.4%** |
| Bytes/op | 208,054 | 130,495 | −37.3% |
| `ListMemories` (Limit 8), control | 64 allocs | 64 allocs | unchanged |

**Target ≥50% allocation reduction: PASS (56.4%).**

## 4. Correctness / safety

- Question-aware tests: 12/12 PASS. Contract/invariant tests: 12/12 PASS.
- No trust check removed; injection/supersession/conflict/sufficiency still run.
- Project scoping and cache generation invalidation unchanged.
- Full learning suite **ok 37.8 s**; race subset **ok 32.0 s**.

## 5. Scale note

Selector allocations are bounded by the retrieved candidate set (CandidateK), not the
corpus size, so the per-retrieval allocation count is stable across 1K/10K/100K/1M
stores. Latency at larger stores is dominated by the SQLite trigram query (shared with
the store design), not the selection layer. A per-scale latency sweep (1K/10K/100K/1M)
is **NOT TESTED** in this pass; the allocation metric — the target of this phase — is
corpus-independent.

## 6. Verdict

**PASS.** Allocation target met with no correctness, trust, or cache-correctness
regression.
