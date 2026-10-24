# Phase 1 — Phase 2 Readiness (22 October 2026 revalidation)

Phase 1 validation now includes both WSL2 and the Apple M5 Mac. The Mac SSH, neural inference, and actual local-agent execution blockers listed in the earlier report below were resolved. The new work remains local; no paid compute, push, or PR was used.

| Component | Phase 1 verdict | Phase 2 entry condition |
|---|---|---|
| Lexical and cache | PASS | Preserve 10K differential-query equivalence in future planner changes. |
| Build/portability | CONDITIONAL PASS | Resolve or classify the remaining repository-wide test failures recorded in `BUILD_AND_COMPATIBILITY.md`. |
| ANN | CONDITIONAL PASS | Improve M64 build cost/allocations while protecting recall; evaluate real embedding distributions. |
| Neural | CONDITIONAL PASS | Use a larger held-out, independently labeled set and repeat across seeds/domains. |
| Trust/security | CONDITIONAL PASS | Calibrate on real reviewed memories; run a live-agent adversarial prompt-injection evaluation. |
| Crash/concurrency | CONDITIONAL PASS | Add interrupted migration, power-loss/storage simulation where possible, long soak and multi-process stress. |
| Token budgeting | CONDITIONAL PASS | Count actual serialized/model tokens and validate critical-evidence retention on a larger set. |
| Real agent | CONDITIONAL PASS | Run many blinded task fixtures and repetitions; compare original/V6 when their retrieval differs. |

The 10K M64 ANN build took 17.18 s and allocated about 5.07 GB cumulatively on Mac; no extrapolated 1M build is claimed from it. The 40-case neural and 50-case trust suites are useful regression seeds, not statistical certification. The three-task agent comparison yielded original=V6=2/3. A V6-specific end-to-end benefit is still unproven. Stop here for review before Phase 2 or large-scale ANN runs.

## Prior Phase 1 report (preserved below)

# Phase 1 — Phase 2 Readiness

## Ready

- Lexical engine: correct (differential PASS) and fast; safe to build on.
- Cache/generation: consistent under race and cross-store tests.
- ANN (corrected HNSW, M=64): recall@10 ≥ 0.99 at small scale; correctness target met.
- Trust rules and scope isolation: correct on the small labeled set.
- Default path: no GPU/model/network; opt-in neural + ANN.

## Prerequisites / blockers before Phase 2

1. **ANN build throughput** — single-threaded, ~5 ms/node at M=64; 1M ≈ 83 min.
   Phase 2 must parallelize construction (with correctness tests) or adopt a
   faster index. This is the top blocker.
2. **Crash-mid-transaction fault injection** — build a subprocess harness that
   kills at defined points and verifies acknowledged-write durability.
3. **Prompt-injection / security smoke harness** — untested.
4. **Mac access** — add the public key to run macOS build + local neural/agent.
5. **Larger held-out set** — 17 cases is too small; target ≥200 for neural and
   ≥50 trust scenarios.
6. **Token retention** — tune the budget so no gold memory is dropped.
7. **Agent harness** — required for the end-to-end gate.

## Explicit non-goals for Phase 2 (until reviewed)

- No 1M ANN M=64 unless build is parallelized.
- No production-readiness claim until crash, security, agent, and scale gates pass.

## Recommended Phase 2 order

1. Parallel ANN build (+ correctness + recall re-validation).
2. Crash fault injection + durability report.
3. Security injection harness.
4. Larger neural/trust datasets.
5. Agent harness (once a machine/model is available).
