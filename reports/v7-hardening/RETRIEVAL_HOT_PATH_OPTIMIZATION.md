# Beacon V7 hardening — Phase 4: Retrieval Hot-Path Optimization

Status: **Implemented (Phase C work, committed) — allocation count reduced without
evidence-quality regression.**

## 1. Profile (warm `SearchQuestion`, 1,000-memory store, CandidateK 50)

pprof showed the question-aware path's allocations were dominated by
`SelectTrustContext` (59.6% cumulative) — specifically `candidatePairs`,
`resolveSupersessions`, and `significantTerms`. The same candidate bodies were tokenized
repeatedly. CPU was dominated by SQLite (the lexical query), not the selection logic.

## 2. Changes (each a focused, general fix)

1. `search_question.go` — per-call caches in `questionIntent`:
   - `termCache`: `memoryTerms` computed once per memory instead of in
     `direct`/`slotMatch`/`primaryMatch`/`linked`.
   - `defCache`: the uncertainty scan runs once per memory, not in both passes.
2. `context_select.go` — `candidatePairs` computes `significantTerms` once per memory and
   uses a direct pairwise scan for ≤64 candidates (the retrieval case), keeping the
   inverted index for large sets; both yield the identical pair set.
3. `context_select.go` — `resolveSupersessions` precomputes terms once (was up to 4× per
   pair).

## 3. Result

| Metric (warm SearchQuestion) | Before | After |
|---|---:|---:|
| Bytes/op | 248,382 | **207,988** (−16%) |
| Allocs/op | 2,099 | **2,007** (−4%) |
| Correctness | — | unchanged |

CPU remains SQLite-bound; wall-clock on this OneDrive/WSL host is noisy and is not used
as the headline metric.

## 4. Constraints honoured

- Question-aware relevance and security enforcement are intact: the full trust pipeline
  (injection, source, supersession, conflict, sufficiency) still runs.
- No evidence deleted, no trust check skipped.
- Evidence-selection tests (12 `TestSearchQuestion*`, 12 `TestContract_*`,
  `TestPhase4DSelectorScaling`) all pass.

## 5. Remaining overhead

The dominant remaining warm cost is the SQLite trigram query (shared with the store
design), not the selection logic. Further allocation reduction would require
restructuring `SelectTrustContext` to thread precomputed term maps across its stages
(invasive); deferred.

## 6. Verdict

**PASS** — meaningful allocation reduction (−16% bytes) with no correctness or safety
regression. p95 not independently re-measured (host noise); allocation counts are the
stable metric.
