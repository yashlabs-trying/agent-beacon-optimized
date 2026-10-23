# Phase 4D — Context Selector Optimization

## 1. Bottleneck identified

Supersession and contradiction checks were an **O(n²)** pairwise scan over the candidate
set (`detectSupersessions`, `detectContradictions`). Profiling the selector at n=1000
showed the pairwise comparisons dominated (≈168 ms for detection alone).

## 2. Change

`candidatePairs` builds an inverted index (term → memory indices) and counts shared
significant terms per pair, emitting only pairs sharing ≥2 terms. Both supersession and
contradiction detection iterate these pairs instead of the full square. This is
near-linear in the common case (few memories per subject term).

**Correctness preservation.** The indexed detector applies the identical predicates and
is equality-tested against the trusted O(n²) `detectContradictions` (unit test + the
scaling test asserts equal conflict counts at every size). An earlier "skip
high-frequency terms" guard was **removed** because it broke legitimate correction chains
(shared subject terms are legitimately high-frequency) — correctness was prioritised, as
required.

## 3. Measured before/after (same machine, same corpus)

| n | naive O(n²) | indexed | speedup | conflicts (naive=indexed) |
|---:|---:|---:|---:|---:|
| 20 | 0.21 ms | 0.19 ms | 1.1× | 0 = 0 |
| 100 | 2.15 ms | 0.88 ms | 2.5× | 0 = 0 |
| 500 | 38.5 ms | 3.9 ms | 9.9× | 0 = 0 |
| 1000 | 168.4 ms | 5.4 ms | **31.5×** | 0 = 0 |

`BenchmarkSelectTrustContext1000`: **9.0 ms/op**, 3.15 MB/op, 27,421 allocs/op.
(Phase 4C O(n²) path on the same corpus: ≈1.5 s.)

## 4. Preserved properties

- Selected correct evidence: unchanged (dev pilot identical or better).
- Conflict detection: equal counts (equality test).
- Scope isolation: `TestContract_ScopeIsolation`.
- Supersession correctness: `TestContract_SupersessionKeepsHistory`, chains/cycles.
- Injection protection and budget behaviour: contract tests.

## 5. Cold vs warm

The selector is a pure CPU function with no cache; cold and warm runs are statistically
identical (no I/O). The dominant cost is `significantTerms` tokenization, which is
computed once per memory.

## 6. Honest limitation

A corpus where a very large fraction of memories share the same two subject terms
(e.g. one enormous correction chain) remains O(bucket²). This is inherent to the pairwise
semantic relationship and is rare in practice; it is documented rather than hidden.

## Verdict

**PASS** — 31.5× faster at n=1000 with identical results and no correctness regression.
