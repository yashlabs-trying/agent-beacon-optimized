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

Mac (192.168.1.10) SSH: `Permission denied (publickey,password,keyboard-interactive)`.
Neither `id_ed25519` nor `runpod_kernellab` is authorized. Neural/agent work on the
Mac is therefore NOT TESTED in Phase 1; neural was instead validated on the GPU pod
in V6 (allowed evidence, no new spend).

## What passed / what is unresolved

Passed: build integrity, lexical correctness, cache consistency, ANN correctness,
trust rules, concurrency, durability of derived-index loss.
Unresolved: crash-mid-transaction, prompt-injection harness, token-retention
tradeoff, agent harness, Mac portability.
