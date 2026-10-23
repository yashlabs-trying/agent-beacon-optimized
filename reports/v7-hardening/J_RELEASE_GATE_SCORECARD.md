# Beacon V7 hardening — Phase J: Release Gate Scorecard

Frozen source for this scorecard: `codex/beacon-v7-hardening` (see FINAL verdict for HEAD).

| # | Gate | Status | Evidence / reason |
|---|---|---|---|
| 1 | No known critical correctness regressions | **PASS (tested scope)** | full learning suite ok 58.0 s; 12 question + 12 contract tests pass; default path unchanged |
| 2 | No known critical security or tenant-isolation bypass | **CONDITIONAL** | injection tests pass; project/cache/vector isolation pass; **authenticated provenance NOT IMPLEMENTED**; cross-tenant auth NOT TESTED |
| 3 | No unresolved data-loss defect | **PASS (tested scope)** | crash/durability suites pass, 0 acknowledged-write loss; 1M/disk-exhaustion NOT TESTED |
| 4 | Retrieval/end-to-end performance comparable to original | **CONDITIONAL** | warm allocations reduced (−16%); wall-clock SQLite-bound/noisy; **cold first-use FTS penalty (Phase A) NOT fixed**; e2e NOT re-measured |
| 5 | Sustained 16-client writer p99 meets budget | **FAIL** | measured 835–935 ms vs 250 ms budget; ≤8-client budget retained |
| 6 | Intelligence improvements survive development validation | **PASS (unit)** | Phase B fix verified; **fresh model-backed held-out validation NOT RUN** |
| 7 | Full applicable test coverage passes | **PARTIAL** | core packages pass; full repo CLI suite, ANN/neural, MCP, cross-platform NOT RUN |
| 8 | Benchmarks reproducible | **PASS** | generator seeds + fixture hashes recorded; benches committed |
| 9 | Final source commit frozen | **PASS** | hardening branch HEAD frozen for this report |
| 10 | Independent 500-task holdout untouched | **PASS** | not generated, inspected, or run |

## Blocking gates

- **Gate 5 (16-client writer p99): FAIL** — pre-existing, unresolved; documented.
- **Gate 2 (provenance): CONDITIONAL** — authenticated provenance not implemented.
- **Gate 4 (cold performance): CONDITIONAL** — cold FTS penalty open.

## Verdict

V7 hardening is **NOT release-ready** for the independent 500-task benchmark under the
strict gate set: gate 5 fails, gates 2 and 4 are conditional, and gate 7 is partial. The
Phase B correctness fix and Phase C allocation reduction are solid, but the performance
and security-provenance gaps must close first.
