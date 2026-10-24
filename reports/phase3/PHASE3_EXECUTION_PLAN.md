# Beacon V6 Phase 3 execution plan — 22 October 2026

This is the plan and resource estimate **before** large Phase 3 experiments.
No Phase 3 ranking, agent, sustained-load, or release experiment has been run
against the newly locked fixtures. The verified V6 integration snapshot and
independent original-Beacon baseline are identified in `FREEZE_MANIFEST.md`.
Phase 1/Phase 2 branches and `reports/integration/raw/` remain untouched.

## Scope and order

| Step | Work | Decision gate |
|---|---|---|
| 0. Reconfirm locks | Verify the frozen integration tag, baseline bundle/clone, fixture hashes, Mac/WSL Git SHAs, Go versions, cached BGE revision, and local Ollama model digest. Record exact commands and outputs. Treat previously scored Phase 1/2 fixtures as development sets. | Any changed hash or baseline SHA stops evaluation until reconciled. |
| 1. Real-neural ranking | Use the cached BGE-small sidecar offline on the Mac. Select weights and thresholds only on already observed tuning data. Evaluate lexical, hashed, exact neural, default hybrid, and semantic-heavy hybrid once on the 48 locked queries; report category-level R@1/R@5, MRR, NDCG@5, p50/p95 latency, memory, and failure behavior. Compare ANN with exact search at bounded 10K and 100K, not 1M. | Neural R@5 must match or exceed hashed and lexical on the locked set, with no exact-identifier regression; p95 query latency under 100 ms and sidecar RSS under 1 GiB on the Mac. ANN must retain Recall@10 ≥0.95 versus exact at both bounded sizes, build 100K within 2 minutes and peak RSS under 1 GiB. Otherwise both paths remain opt-in. |
| 2. Agent task success | First make the evaluator load the six locked tasks while keeping objective tests outside the agent-readable workspace. Run paired no-memory, original-Beacon, and V6 arms with identical local model, prompt, tool limits, seed, and isolated project/database state. Use this six-task set to validate the harness, not to claim superiority. Curate and lock at least 30 additional tasks under a new ID before a three-seed final comparison; randomize arm order and preserve per-run retrieval IDs and tool traces. | A V6 advantage requires a positive lower bound on the paired success-rate difference versus original Beacon, with no critical-task regression. If the interval includes zero, report parity. The prior 3-task 2/3-versus-2/3 smoke remains only a smoke result. |
| 3. Sustained writer latency | On WSL2, then Mac, run separate steady-state and forced-index-rebuild workloads at 1/4/8/16 clients using disposable databases. Retain read/write/supersession throughput, p50/p95/p99, busy/stale/error counts, WAL, RSS, and goroutine trends. Re-run process-crash durability after any concurrency fix. Do not alter the SQLite busy-timeout fix. | Zero acknowledged-write loss, cross-project leak, or non-busy error; no sustained resource growth. Target steady-state read/write p99 <250 ms at the chosen supported client count and forced-rebuild p99 <1 s. If 16 clients misses, publish a lower operational client budget rather than claiming 16-client support. |
| 4. Contradiction and token evidence | Tune detector and context selector only on the seen Phase 1/2 matrices. Evaluate the 24 new labeled contradiction pairs and the distinct-finding labels in the 48-query retrieval/context set once after choices are fixed. Report category failures, precision/recall, cross-talk, findings retained, token count, and latency. Expand and lock a separate larger final set before any broad claim. | Contradiction precision ≥0.95, recall ≥0.90, and zero project/version boundary violations on the locked cases. A default token-saving selector must retain every distinct finding retained by unbudgeted top-five while saving ≥20% tokens; otherwise keep budgeting opt-in and prefer measured evidence deduplication. |
| 5. Final local release gate | Rebuild embedded hooks, build/vet all changed Go modules, run full CLI and consumer suites, race tests, differential search, both crash harnesses, security/trust, real-neural opt-in, ANN correctness, and offline/no-sidecar fallback on Mac and WSL2. Include the repository's local packaging smoke checks. Audit defaults and reproducibility, then issue a release-readiness report with exact SHAs and residual limits. | No failing required test or unexplained regression. CLI/MCP/dashboard must remain offline lexical by default. A default change for neural or ANN requires the earlier quality and operational gates; otherwise leave them opt-in. |

## Controls on evaluation

- The corrected Phase 2 hashed-ranking result is R@1 **0.208 semantic versus
  0.167 default RRF**, not the 0.812/0.729 in the original summary. Use the
  corrected table and committed raw log for comparisons.
- Run tuning on the already observed Phase 1/2 fixtures. Record parameters
  before opening a new held-out set. Once a held-out set informs a fix, mark it
  observed and lock a new dataset ID for the final gate.
- Report exact model revisions, Go version, OS, machine, corpus size, seed,
  candidate limit, token estimator, and p50/p95/p99 method for every result.
- Keep original and V6 agent arms on the same Apple M5, sequentially or in
  randomized order without simultaneous benchmark/model load. Keep sidecar
  and Ollama runs separate for comparable memory and latency measurements.
- Use only the existing Mac and WSL2 laptop. No Runpod, cloud model/API,
  paid sandbox, new hardware, 1M ANN run, push, or PR is part of this plan.

## Estimated local resources

| Work | Host | Compute time | Peak incremental RAM | Disk / notes |
|---|---|---:|---:|---|
| Real-neural ranking and bounded ANN | Apple M5, 10 logical CPUs, 16 GiB unified memory | 30–90 minutes including repeated 10K/100K builds and analysis | BGE sidecar about 0.6 GiB; bound Go + sidecar to <2 GiB beyond OS | Existing cached BGE model; <1 GiB of new logs/temporary indexes. |
| Agent harness pilot and 30-task/three-seed paired evaluation | Same Mac, local `qwen2.5-coder:7b` | Pilot 20–40 minutes; final 2–4 hours for 270 arm-runs plus triage | Existing 4.7 GiB quantized model; reserve 6–8 GiB total for Ollama and fixtures | Existing model cache; roughly 0.5 GiB for isolated workspaces/traces. |
| Writer and crash tests | WSL2 laptop (8 vCPU/about 8 GiB RAM), then Mac | 60–120 minutes total for 15-minute windows, rebuild cases, and reruns | Target <4 GiB per host | Disposable SQLite DB/WAL up to about 1 GiB; clean only verified test paths. |
| Contradiction/token evaluation | WSL2 or Mac | <30 minutes compute, plus review time | <1 GiB | Small locked JSON and result logs. |
| Final local release validation | Both hosts | 45–90 minutes, with race tests dominating | WSL2 within its 8 GiB limit; Mac within 16 GiB | Allow about 1 GiB for build cache/logs. |

The phases are sequential where they share the Mac. Budget roughly **one
working day of elapsed local execution and review**, with zero paid compute.
These are planning estimates, not benchmark measurements; run durations and
resource peaks will be recorded before any default or release decision.

## Current stopping point

The integration freeze, baseline bundle, corrected Phase 2 report, and new
held-out fixture hashes are ready for review. Phase 3 experiments have not
started. Neural retrieval and ANN remain opt-in, and the earlier verified
branches have not moved.
