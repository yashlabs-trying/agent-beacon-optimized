# Beacon V6 — Phase 4A Executive Results

Rigorous benchmarking, independent baselines, and real-agent evaluation of the
integrated V6 memory engine.

- Date: 2026-10-23 (Asia/Kolkata)
- Branch: `phase4a/beacon-v6-benchmark` (isolated worktree `/home/<user>/work/phase4a`)
- Base: Phase 3D `bcf3a1fd46d5865ca21b48a224646b9e107b2f86`
- Frozen refs unchanged: integration `209a987b…`, prep `c9275d84…`, baseline `99c70908…`
- Execution hosts: WSL2 laptop (`lappiiii`, 8 vCPU, 7.6 GiB, Go 1.27.3 linux/amd64)
  **and the Apple M5 Mac (`192.0.2.10`, 16 GiB, MPS GPU, Ollama 0.40.6)** over SSH.
- No paid compute, no GPU rental, no model downloads, no push/PR. All model weights
  (BGE-small, qwen2.5-coder) were already cached locally.

**Bottom line.** Phase 4A unblocked the two Phase 3D gates that were previously
`BLOCKED` — real BGE-small neural retrieval and the model-dependent six-task agent
pilot — because the Mac is now reachable. Results: (1) the real neural arm runs and
clearly leads retrieval (R@5 **0.979** vs hashed 0.875 vs lexical 0.167); (2) a real
coding agent reaches **6/6 with either Beacon** vs **4/6 with no memory**, but **V6 does
not beat original Beacon** (both 6/6, identical retrieved evidence); (3) the evaluation
harness itself passes all 16 validation checks, including negative controls. The most
important *negative* finding: on the locked set a trivial **SQLite FTS5 baseline
(R@5 0.917) beats V6's hashed-semantic arm (0.875)** — V6's only demonstrated retrieval
win over simple baselines comes from the real neural model.

---

## 1. Provenance and integrity

| Item | Value |
|---|---|
| Phase 3D base | `bcf3a1fd46d5865ca21b48a224646b9e107b2f86` |
| Ancestry verified | `99c70908 → 209a987b → c9275d84 → c8a6749b → 17a10b0f → d12e413a → bcf3a1fd` (all `merge-base --is-ancestor` = 0) |
| Original Beacon | `99c709081ad49da1c5ade205929b61b19360ed9b` |
| Locked fixtures | re-verified `LOCK OK` ×3 **before and after** all evaluation (see §4) |

The provenance strings in the Phase 4A brief (Phase 3A–3D and V6-integration commits)
were located and verified: they exist in the primary repository (git common dir
`C:\\Users\\<user>\\work\rescue\beacon`), whose worktrees `phase3a`–`phase3d`
and `beacon-integration` carry them. An earlier scan of a separate Mac clone
(`~/beacon-phase1-v6`) did not contain them; that clone is a Phase 1/2-only checkout and
was not used for evaluation.

---

## 2. Gate summary

| Benchmark | Gate | Result |
|---|---|---|
| A | Harness detects injected failures; passes isolation, determinism, negative controls | **PASS** (16/16) |
| B | Independent retrieval baselines measured on identical corpus/queries/labels | **PASS** |
| B | V6 hashed/hybrid beats a simple independent baseline | **FAIL** (FTS5 0.917 > hashed 0.875) |
| B | Real BGE neural beats lexical/hashed and independent baselines | **PASS** (0.979 > 0.917) |
| C | Local model prerequisites present and capability-verified | **PASS** (qwen2.5-coder:7b suitable) |
| D | Six-task paired pilot executed with a real model | **PASS (executed)** |
| D | V6 > original Beacon on agent task success | **FAIL / NOT DEMONSTRATED** (6/6 = 6/6) |
| D | Memory > no memory on agent task success | **PASS (descriptive)** (6/6 vs 4/6, n=6) |
| E | ≥30-task expanded protocol designed and frozen before model runs | **PASS (protocol)** |
| F | Reliability/adversarial suite | **PASS** (full learning suite, targeted race, hnsw) |
| F | 16-client write p99 < 250 ms | **FAIL** (unchanged; ≤8-client budget retained) |
| — | Contradiction default recall ≥ 0.90 | **FAIL** (0.500 locked; cross-kind opt-in 1.000) |
| — | Neural/ANN/hybrid/token-budgeting remain opt-in | **PASS** (no default changed) |

---

## 3. Headline numbers

### Retrieval on the locked 48-query fixture (identical corpus, labels, limit=10)

| Method | R@1 | R@5 | R@10 | MRR | nDCG@5 | p50 |
|---|---:|---:|---:|---:|---:|---:|
| Original lexical (`99c70908`) | 0.167 | 0.167 | — | 0.167 | 0.134 | 0.54 ms |
| V6 lexical | 0.167 | 0.167 | — | 0.167 | 0.134 | 0.57 ms |
| V6 hashed-semantic | 0.688 | 0.875 | — | 0.764 | 0.716 | 1.1 ms |
| V6 hybrid RRF / ANN | 0.688 | 0.875 | — | 0.764 | 0.716 | 1.0 ms |
| **V6 real neural BGE-small (Mac MPS)** | **0.646** | **0.979** | 1.000 | **0.785** | **0.789** | 43.6 ms¹ |
| Independent BM25 | 0.688 | 0.875 | 0.979 | 0.773 | 0.735 | 0.13 ms |
| Independent SQLite FTS5 | 0.708 | 0.917 | 0.958 | 0.788 | 0.753 | 0.28 ms |
| Independent BGE exact (cosine oracle) | 0.750 | 1.000 | 1.000 | 0.848 | 0.863 | 57.6 ms¹ |
| Recency-only | 0.000 | 0.083 | 0.167 | 0.040 | 0.048 | 0.001 ms |

¹ Includes LAN round-trips to the Mac sidecar; not an on-host latency.

### Six-task paired agent pilot (real model: qwen2.5-coder:7b, Ollama on M5)

| Arm | Task success | Avg agent s | Prompt tok | Gen tok | Avg retrieval ms |
|---|---:|---:|---:|---:|---:|
| A no memory | **4/6** | 13.0 | 29,123 | 1,468 | 0 |
| B original Beacon | **6/6** | 14.6 | 28,728 | 1,730 | 13.7 |
| C Beacon V6 | **6/6** | 15.0 | 28,733 | 1,730 | 13.8 |

Memory arms retrieved identical IDs (`<task>-distractor, <task>-current`), so B and C
differ only in CLI implementation, not in retrieved evidence.

### Reliability (fresh on WSL2, Phase 4A)

- `go test ./internal/learning/` → **ok 48.9 s**
- `go test -race ./internal/learning/` (concurrency/contention subset) → **ok 28.5 s**
- `go test ./internal/hnsw/` → **ok 0.5 s**
- Locked contradiction default: precision **1.000**, recall **0.500**; opt-in cross-kind recall 1.000.

---

## 4. Locked-fixture integrity

`python3 reports/phase3/heldout/validate_lock.py` (before and after evaluation):

```
LOCK OK retrieval_context_v1.json FADF313E35F7E41FBC21CFB180FE88075EAC6485B6FAD851AA2CBD8F9503114A
LOCK OK contradiction_v1.json     B3B4CAC0A7E8CC072482E8C35344F9239AFA7B4EA6C255861B2037FFB1448E84
LOCK OK agent_tasks_v1.json       409195B01ED6E2DB55BCDF4EF5D8710DEEEDFBFFE5320EB4D5722215AB0DC530
```

No locked fixture, expected outcome, frozen reference, or default was modified.

---

## 5. Decision criteria

1. **Is V6 retrieval objectively stronger than original Beacon under comparable
   conditions?** Yes, for the semantic arms: hashed/hybrid R@5 0.875 and neural 0.979
   vs original lexical 0.167 on the same fixture. Caveat: V6's *lexical* path is
   byte-for-byte equivalent to the original (both use the same AND-substring predicate);
   the gain comes entirely from the added (opt-in) semantic layer.
2. **Does V6 retrieve correct evidence within an acceptable latency and token budget?**
   Hashed/hybrid: yes (≈1 ms, sub-2 ms p95). Neural: 0.979 R@5 but 43.6 ms p50 over the
   LAN (dominated by network + MPS warm-up; an on-host sidecar would be lower). Token
   budget is unchanged (1,242 tokens / 17 q); budgeting is opt-in and loses 3/109
   findings.
3. **Can the benchmark genuinely distinguish correct, incorrect, and irrelevant memory?**
   Yes — 16/16 harness checks pass, including incorrect-memory failure, citation-not-
   rewarded, plausible-but-wrong failure, failed-tool-call detection, isolation,
   determinism, and a corrupted-scorer negative control.
4. **Does a real coding agent achieve better functional outcomes with V6?** Not
   demonstrated. Both Beacon arms scored 6/6; V6 showed no advantage over original
   Beacon because retrieval was identical. Memory *did* help over no memory (6/6 vs 4/6)
   but n=6 cannot support a statistical claim.
5. **What prevents production adoption or credible founder outreach?** (a) No
   demonstrated V6-over-original agent advantage; (b) the hashed arm is not better than a
   trivial FTS5 baseline; (c) neural needs a sidecar + model and is 43 ms/query; (d) ANN
   100K build ≈ 102 s and 1M M=64 ≈ 83 min; (e) 16-client write p99 744–795 ms; (f)
   contradiction default recall 0.50; (g) token budgeting loses findings. None of these
   block a *research* claim; all block a *production/adoption* claim.

---

## 6. Recommendation

**Do not invest further in the hashed-semantic arm** — it is matched by an off-the-shelf
SQLite FTS5 index. Focus on:

1. **Model-based outcome validation (highest priority).** Run the frozen ≥30-task,
   three-seed protocol (`EXPANDED_EVALUATION_PROTOCOL.md`) comparing none / original /
   V6-**neural** with qwen2.5-coder:7b. This is the only experiment that can answer
   whether better retrieval yields better agent outcomes.
2. **Ranking and memory selection** — the lexical arm returns the gold doc only 16.7% of
   the time; FTS5 already shows that a better free ranker exists.
3. Keep neural/ANN/hybrid/token-budgeting/cross-kind contradiction **opt-in** until the
   ≥30-task study justifies promotion.

**Highest-priority next experiment:** the ≥30-task held-out three-seed paired pilot
(none vs original vs V6-neural), with per-task retrieval-ID and token traces, scoring
functional success only.

---

## 7. Artifacts

- Reports: `reports/phase4/part1/*.md`
- Raw evidence: `reports/phase4/part1/RAW_EVIDENCE/` (binaries, JSON, txt)
- Harnesses: `reports/phase4/part1/harness/` (`validate_harness.py`,
  `retrieval_baselines.py`, `model_capability_check.py`, `embed_sidecar_host.py`)
- Reproduction: `reports/phase4/part1/run_phase4a.sh`
- Harness improvement: `reports/phase3/part4/harness/agent_eval.py` (added `--ollama-url`
  and a multi-object JSON-in-content fallback; both documented and regression-safe)
