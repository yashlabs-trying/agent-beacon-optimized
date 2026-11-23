# Phase 1 — Executive Results (22 October 2026 revalidation)

Recovered local branch `feat/beacon-memory-engine-v6-production` at `85e6ea1cbb25ac193bd902966aa5889bf174dfa3`. The ordered V6 ancestry contains `519f6b49`, `09165a03`, `aee750a5`, `551ef0b6`, and `85e6ea1c`. Upstream comparison baseline is `99c709081ad49da1c5ade205929b61b19360ed9b`. A complete Git bundle preserved the unpublished history before isolated WSL2 and Mac clones were made. Both clones matched the recovered HEAD. This report includes subsequently repaired Phase 1 changes; the final local commit records their exact SHA.

The revalidation used Ubuntu WSL2 on the Windows laptop (8 vCPU, about 8 GiB RAM) and an Apple M5 Mac (16 GiB unified memory). Go 1.27.3 was installed locally on both. The Mac ran local MPS neural inference and Ollama models. No paid compute was used, and nothing was pushed.

| Gate | Verdict | New evidence and limit |
|---|---|---|
| Build and compatibility | CONDITIONAL PASS | Go build/vet and learning tests pass on both. Repository-wide results are in `BUILD_AND_COMPATIBILITY.md`. |
| Lexical correctness and cache | PASS | 10,000 randomized differential queries and cache/index recovery tests pass; same-Mac 10K no-match benchmark 16.64 ms baseline versus 0.306 ms V6. |
| ANN | CONDITIONAL PASS | Twelve-cell 100–10K sweep; M=64 recall@10 0.9953–1.0000, but M=16 drops to 0.7516 at 5K and M=64 build costs 17.18 s at 10K. |
| Neural retrieval | CONDITIONAL PASS | Real BGE-small on Mac MPS, 40 labeled cases: neural R@5 36/40 versus hashed 29/40 and lexical 3/40; small synthetic sample. |
| Trust/provenance | CONDITIONAL PASS | 50 labeled scenarios pass after version-scope repair; English rules remain heuristic. |
| Security retrieval smoke | CONDITIONAL PASS | Synthetic hostile memories remain data and cross-project isolation holds; no real-agent injection trial. |
| Crash recovery | CONDITIONAL PASS | Six subprocess process-exit checkpoints preserve atomicity; no power-loss or interrupted-migration test. |
| Concurrency | CONDITIONAL PASS | 20-cell WSL 1/4/8/16-client sweep, zero errors; former 5 s busy timeout reproduced an acknowledged-write failure and was raised to 30 s. Long soak remains open. |
| Token budgeting | CONDITIONAL PASS | Fixed 2,140 estimated tokens vs budgeted 1,358, with gold 12/17 in both after fixing returned-body truncation. Estimate omits metadata. |
| Real coding agent | CONDITIONAL PASS | Local 7B model made actual file/test tool calls: no memory 1/3, original 2/3, V6 2/3. Original and V6 retrieved the same notes; no comparative V6 task-success gain shown. |

The prior Phase 1 report at `aee750a5` used WSL2 and an earlier GPU pod. Its 17-case neural R@5 (lexical 0.412, hashed 0.706, neural 1.000), 1M lexical timings, and earlier ANN measurements are historical evidence, **not** Mac/2026 reproductions. Its Mac SSH and agent blockers were resolved in this revalidation; the prior report is retained below for provenance. See each component report and raw logs in `reports/phase1/artifacts/` for commands, seeds, and limits.

Phase 2 should wait for larger held-out retrieval and agent sets, ANN construction work, sustained durability/concurrency tests, and agent-level prompt-injection evaluation. This is a Phase 1 result, not a production-readiness claim.

## Prior Phase 1 report (preserved verbatim below)

# Phase 1 — Executive Results

Branch `feat/beacon-memory-engine-v6-production`. Revision tested: `aee750a5`
(plus the Phase 1 test scripts). Hardware: WSL2 Ubuntu (i5, 8 vCPU, 8 GB). Mac
(Apple Silicon, 16 GB) was **not reachable** — see blocker below.

## Gates

| Gate | Status |
|---|---|
| BUILD/CODE QUALITY | **PASS** |
| LEXICAL CORRECTNESS | **PASS** |
| CACHE CONSISTENCY | **PASS** |
| ANN CORRECTNESS | **PASS** (M=64) |
| NEURAL INTEGRATION | **CONDITIONAL PASS** (validated on GPU pod; Mac blocked) |
| TRUST/PROVENANCE | **PASS** (small n) |
| SECURITY SMOKE | **NOT TESTED** (no injection harness) |
| CRASH RECOVERY | **CONDITIONAL PASS** (index-drop recovery; crash-mid-tx not injected) |
| CONCURRENCY | **PASS** |
| TOKEN SELECTION | **CONDITIONAL PASS** (1.72× fewer tokens, 1/17 gold lost) |
| REAL AGENT HARNESS | **NOT TESTED** |

## Headline measurements (WSL2 unless noted)

- gofmt clean; `go build`/`go vet` OK; full `go test` PASS; `go test -race` PASS (643.8 s).
- Lexical differential correctness: 10,000 randomized queries identical to the original scan.
- ANN small scale: M=64 recall@10 **1.000** (100/5K/10K), 0.991 (1K); M=32 ~0.97–0.99.
- Trust: contradiction precision/recall **1.00** (8 pairs); scope leakage **0**.
- Token budgeting: 2,140 → 1,242 tokens (1.72×), gold retained 11/17 vs 12/17.
- Neural (GPU pod, bge-small): recall@5 **1.000** vs hashed 0.706 (17-case set).

## Blocker

Mac (192.0.2.10) SSH: `Permission denied (publickey,password,keyboard-interactive)`.
Neither `id_ed25519` nor `runpod_kernellab` is authorized. Neural/agent work on the
Mac is therefore NOT TESTED in Phase 1; neural was instead validated on the GPU pod
in V6 (allowed evidence, no new spend).

## What passed / what is unresolved

Passed: build integrity, lexical correctness, cache consistency, ANN correctness,
trust rules, concurrency, durability of derived-index loss.
Unresolved: crash-mid-transaction, prompt-injection harness, token-retention
tradeoff, agent harness, Mac portability.
