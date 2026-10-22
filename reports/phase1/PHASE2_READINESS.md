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
