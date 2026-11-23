# Phase 5 — Executive Summary

Ultimate adversarial evaluation of optimized V6 vs untouched original Beacon on a frozen
300-task held-out suite.

- Date: 2026-10-23 (Asia/Kolkata)
- Branch: `phase5/beacon-ultimate-eval` (worktree `/home/<user>/work/phase5`), base `7c7e2493`
- Original Beacon: `99c709081ad49da1c5ade205929b61b19360ed9b` (frozen)
- Hosts: WSL2 laptop + Apple M5 Mac (`192.0.2.10`, Ollama qwen2.5-coder:7b, BGE sidecar)
- No paid compute, no downloads, no push/PR.

**Bottom line (truth, not favorable):** On the frozen 300-task suite, **V6 intelligent
memory (224/300) does not beat original Beacon (229/300)** — they are statistically tied
(paired 17–22–261, exact McNemar **p=0.52**), with original marginally ahead. V6 intelligent
**does** significantly beat V6 semantic retrieval (224 vs 209; 15–0–285, **p=0.0001**), and
**V6 semantic retrieval alone is significantly worse than original Beacon** (209 vs 229;
11–31–258, **p=0.0029**). Memory of any kind massively beats no memory (B vs A 199–0–101,
p<1e-4). The oracle upper bound is 246/300 (0.82), so ~18% of tasks remain unsolved by any
memory configuration.

**Why:** the three configurations differ on a few categories, not uniformly.
- Semantic retrieval retrieves *more* context, which **hurts precision** on fact-lookup
  tasks where lexical's exact matching is an advantage (A3: original 20/20 vs semantic 2/20).
- The intelligent layer **fixes the security regression** semantic introduces
  (B5 trust/adversarial: original 14/20, semantic 5/20, intelligent **20/20** = oracle).
- Semantic retrieval **helps multi-hop** (B3: original 0/20 vs semantic/intelligent 8/20).

**Net:** the intelligent layer recovers most of semantic's losses but not all, landing
essentially even with the original lexical baseline — while being ~22% faster per task
(0.92 s vs 1.18 s) but using ~14% more prompt tokens.

## Headline table (task-level, seed 0; seeds 1–2 identical)

| Arm | Success | Acc | 95% CI |
|---|---:|---:|---|
| A no memory | 30/300 | 0.100 | [0.071, 0.139] |
| B original Beacon | **229/300** | **0.763** | [0.712, 0.808] |
| C V6 semantic | 209/300 | 0.697 | [0.642, 0.746] |
| D V6 intelligent | 224/300 | 0.747 | [0.695, 0.793] |
| E oracle | 246/300 | 0.820 | [0.773, 0.859] |

## Paired comparisons (task-level)

| Comparison | A / B / ties | p |
|---|---|---:|
| D intelligent vs B original | 17 / 22 / 261 | 0.5224 |
| D intelligent vs C semantic | 15 / 0 / 285 | **0.0001** |
| C semantic vs B original | 11 / 31 / 258 | **0.0029** |
| D intelligent vs A none | 198 / 4 / 98 | **0.0000** |
| D intelligent vs E oracle | 6 / 28 / 266 | **0.0002** |
| B original vs A none | 199 / 0 / 101 | **0.0000** |

## Required comparison table

| Dimension | No Memory | Original | V6 Semantic | V6 Intelligent | Oracle |
|---|---|---|---|---|---|
| Task success (300) | 30/300 | **229/300** | 209/300 | 224/300 | 246/300 |
| Contradiction accuracy (B1) | 0/20 | 8/20 | 8/20 | 8/20 | 8/20 |
| Ambiguity handling (B2) | 10/20 | 20/20 | 16/20 | 16/20 | 20/20 |
| Evidence correctness (recall@K) | 0.067 | 0.967 | 1.000 | 1.000 | 1.000 |
| Learning from feedback (A1) | 0/20 | 2/20 | 5/20 | 5/20 | 0/20 (invalid) |
| Recovery (A4) | 0/20 | 14/20 | 14/20 | 14/20 | 20/20 |
| Coding success (C1) | 0/20 | 13/20 | 13/20 | 13/20 | 13/20 |
| Security violations (B5+C3 leaks) | 0/40 | 0/40 | 0/40 | 0/40 | 0/40 |
| End-to-end mean latency | 0.89 s | 1.18 s | 1.13 s | **0.92 s** | 0.90 s |
| Tokens per success | 113/task | 131/task | 151/task | 149/task | 126/task |
| Cost per success | N/A | local | local | local | local |

`local` = no monetary cost (local model). `NOT TESTED` items are flagged in the reports.

## Verdict per dimension

| Dimension | Verdict |
|---|---|
| V6 intelligent vs original (task success) | **EQUIVALENT** (tied, p=0.52) |
| V6 intelligent vs V6 semantic | **BETTER** (p=0.0001) |
| V6 semantic vs original | **WORSE** (p=0.0029) |
| Memory vs no memory | **BETTER** (p<1e-4) |
| Contradiction resolution | **EQUIVALENT** (all memory arms 8/20; model ceiling) |
| Ambiguity | **WORSE for semantic/intelligent** than original (B2 20 vs 16) |
| Evidence grounding | **EQUIVALENT/BETTER** (recall 1.000 vs 0.967) |
| Learning from feedback | **NOT ESTABLISHED** (A1 invalid; oracle 0/20) |
| Recovery | **EQUIVALENT** (A4 14/20 all memory arms; storage suites PASS) |
| Coding | **EQUIVALENT** (13/20 all memory arms) |
| Security (no leaks) | **EQUIVALENT** (0 leaks all arms; D fixes B5 accuracy) |
| Latency | **BETTER** for D (0.92 vs 1.18 s) |
| Tokens | **WORSE** for C/D (149–151 vs 131) |
| Scalability / concurrency | see SCALABILITY.md |
| V6 vs original overall | **EQUIVALENT / marginally WORSE** |

## Answer to the central question

**Is V6 more useful to a real agent than original Beacon? No measurable advantage on this
suite.** V6 intelligent is statistically tied with original and marginally behind
(224 vs 229). V6's genuine, significant win is over its own semantic arm and in the
security sub-category; its genuine loss is on fact-lookup precision where semantic
over-retrieval hurts. The claim that V6 is "more useful" is **not supported**.

The highest-impact next steps are in `FINAL_VERDICT.md`; the 25 most important verified
failures/regressions are in `TOP_25_REGRESSIONS.md`.

## Coverage

| Item | Status |
|---|---|
| 300 held-out tasks frozen | PASS |
| 5-arm comparison, seed 0 (1,500 runs) | PASS |
| 3 seeds | seeds 1–2 run on a 100-task subset; identical (temp 0 → deterministic) |
| Selector scaling / retrieval scale | PASS (1K/10K/100K) |
| Concurrency / crash / durability | PASS (reused suites) |
| 1M memories, 32/64 clients | NOT TESTED (host limits) |
| Published external benchmarks | NOT TESTED (no local datasets; downloads not authorized) |
