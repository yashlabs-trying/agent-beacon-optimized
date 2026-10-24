# Phase 4D — Integrated Regression Results

Cross-component interaction tests (`context_contract_test.go`), plus the existing suites.
Every interaction test passes.

## 1. Interaction contract tests (12/12 PASS)

| # | Interaction | Invariant | Test |
|---|---|---|---|
| 1 | Contradiction × Supersession | old superseded, history retained | `TestContract_SupersessionKeepsHistory` |
| 2 | Trust × Supersession | untrusted newer cannot override verified | `TestContract_UntrustedCannotSupersede` |
| 3 | Trust × Injection | malicious memory gets no authority | `TestContract_InjectionNoAuthority` |
| 4 | Budget × Contradiction | disputed-fact signal preserved | `TestContract_BudgetKeepsConflictSignal` |
| 5 | Budget × Supersession | superseded fact not resurrected | `TestContract_BudgetDoesNotResurrectSuperseded` |
| 6 | Scope × Semantic similarity | similarity cannot bypass project boundary | `TestContract_ScopeIsolation` |
| 7 | Ambiguity × Retrieval | high relevance ≠ sufficient | `TestContract_InsufficientEvidence` |
| 8 | Ambiguity × Oracle | sufficient evidence not abstained | `TestContract_SufficientEvidence` |
| 9 | Caching × Memory updates | trust-metadata change takes effect | `TestContract_TrustMetadataChangeTakesEffect` |
| 10 | Optimization × Reproducibility | deterministic selection | `TestContract_Deterministic` |
| 11 | Failure × Security | malformed memory not promoted | `TestContract_MalformedNotTrusted` |
| 12 | Indexed × Naive detector equality | optimization cannot change detection | `TestContract_IndexedEqualsNaiveDetector` |

Parallel × Persistence is covered by the storage race/concurrency suites (below); the
selector holds no shared state.

## 2. Suites

| Suite | Command | Result |
|---|---|---|
| Full learning | `go test ./internal/learning/ -count=1` | **ok 67.8 s** |
| Race subset | `go test -race ./internal/learning/ -run 'Concurrency\|Concurrent\|PutMemoryWaits\|SearchConcurrent'` | **ok 42.9 s** |
| HNSW | `go test ./internal/hnsw/` | ok |
| cmd (memory) | `go test ./cmd/ -run Memory` | ok |
| gofmt | `gofmt -l internal/learning cmd` | clean |
| vet | `go vet ./internal/learning ./cmd` | clean |
| build | `go build ./...` | pass |
| Layer unit tests | `-run 'TestSelectContext_\|TestPhase4CMemoryHistory'` | PASS |
| Memory-history 10/50/100 sessions | `-run TestPhase4CMemoryHistory` | PASS |

## 3. Crash / durability / security (reused, unchanged)

Crash at 7 tx points, crash during index rebuild/migration/supersession, process-kill
recovery, cross-scope leakage, hostile content, unapproved-evidence invisibility — all
PASS (Phase 3/4A tests, unchanged by Phase 4D).

## 4. Interaction conflicts found and resolved

- **Sufficiency withholding × resolvable ambiguity** produced 2 regressions on the first
  attempt. Resolved by adding light stemming and lowering the coverage threshold; re-tested
  to 0 regressions. (Documented in `INTELLIGENCE_AND_AMBIGUITY_FIXES.md`.)
- **Indexed candidate discovery × correction chains**: an early high-frequency-term guard
  broke chains; removed. Correctness prioritised over speed.

## Verdict

Integrated regression: **PASS** — no component interaction introduced a regression.
