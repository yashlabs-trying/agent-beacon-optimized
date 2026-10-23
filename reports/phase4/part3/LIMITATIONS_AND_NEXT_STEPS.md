# Phase 4C — Limitations and Next Steps

## 1. What this phase established

- A deterministic, opt-in **trust-aware / supersession-aware context-selection layer**
  that improves agent task success over raw semantic retrieval (dev 20→26/35, paired 6–0,
  p=0.031; held-out 10→14/18, paired 4–0, p=0.125) and reaches **oracle parity**.
- It **eliminates observed prompt-injection success** (4/6 → 0/6 dev; 3/4 → 0/4 held-out)
  while retaining legitimate evidence.
- It is **cheaper**: fewer context tokens (93 vs 98) and lower latency per correct task.
- Trust filtering and injection defense carry the gain; supersession/conflict matter only
  in combination.

## 2. What is NOT established (honest scope)

| Item | Status | Why |
|---|---|---|
| Held-out superiority | **CONDITIONAL** | n=18, one seed, p=0.125 |
| ≥100-task / 3-seed study | **NOT RUN** | compute |
| 1M corpus, 32/64-client concurrency | **BLOCKED** | RAM/time |
| Context-budget matrix (256–2048) | **BLOCKED** | not run |
| Agent-outcome scaling vs history size (E3) | **BLOCKED** | compute |
| Encoded/obfuscated injection | **NOT RUN** | keyword rules only |
| Published benchmarks (LongMemEval/LoCoMo/MemoryAgentBench) | **NOT RUN** | no local dataset; downloads not authorised |
| Independent (non-Beacon) memory competitor | **NOT RUN** | not provisioned |

## 3. Unresolved defects

1. **Unresolvable ambiguity over-answers.** Memory arms guess a wrong value where a
   memory-less agent abstains (`am_unresolvable`). *Fix:* the layer should emit an
   explicit insufficiency signal that suppresses guessing.
2. **Model reasoning ceiling.** `cx_cross_kind`, `cx_negation`, `cx_version_specific`,
   `cx_temporal_obsolete`, and the controls fail for **all** arms including oracle —
   model failures, not memory failures. A stronger model is needed to measure the layer's
   ceiling.
3. **Supersession is lexical.** Paraphrase corrections with low overlap are missed
   (`cx_temporal_obsolete` required a fixture rewrite to be detected).
4. **O(n²) supersession scan** (bounded by K today; 44.7 ms at n=100). Optimise by
   indexing on shared subject terms.
5. **Trust depends on provenance metadata** (`source:` tags) that a real deployment must
   supply; the benchmark simplifies this.
6. **Invalid controls.** The note-only prompt makes the model abstain on
   memory-independent tasks; controls need a prompt permitting general knowledge.

## 4. Recommended next engineering experiment (highest priority)

**A pre-registered ≥100-task, 3-seed paired held-out study** comparing none / original /
v6_semantic / v6_intelligent / oracle, with:
- the unresolvable-ambiguity suppression fix,
- a prompt variant that separates "answer from memory" from "answer from general
  knowledge" so controls are valid,
- per-task retrieval-ID and token traces,
- paired analysis treating seeds as dependent within task.

Do **not** promote any mechanism to default until that study shows a significant,
regression-free improvement. Keep the layer opt-in.

## 5. Secondary next steps

- Add a semantic (embedding-based) supersession/conflict signal to catch paraphrases.
- Optimise the O(n²) selection scan.
- Combine the trust layer with the token-budget selector and sweep budgets.
- Integrate one independent memory system (e.g. a plain RAG/BM25 baseline) as an external
  competitor.
