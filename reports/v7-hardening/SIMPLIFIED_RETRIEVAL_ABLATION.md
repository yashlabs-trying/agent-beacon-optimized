# Beacon V7 hardening — Phase 5: Simplified Retrieval Ablation

Status: **NOT TESTED (decision deferred).** The default V7 path is already lexical-first;
the conditional-semantic ablation was not re-run in this pass.

## 1. Current default behaviour (already lexical-first)

- Default CLI (`memory list` without flags) → `ListMemories` (lexical). No embedder, no
  ANN, no neural.
- `--question-aware` → `SearchQuestion` lexical path; semantic expansion runs **only**
  when trusted lexical evidence does not directly support the question and an embedder is
  configured (`--semantic`).
- ANN and neural (BGE) are **disabled by default**.

So the architecture is already "lexical-first, simple by default." The open question is
whether the *conditional semantic* branch earns its keep.

## 2. Available (historical, not fresh) evidence

| Configuration | Result | Source |
|---|---|---|
| Original | 40/56 | V7 pilot (historical) |
| V7 initial | 51/56 | V7 pilot (historical) |
| V7 semantic-disabled | 48/56 | V7 pilot (historical) |
| V7 post-feedback | 56/56 | same-fixture regression, **not independent** |

The pilot is seven repeated mechanism families; conditional semantic contributed to seven
two-hop wins in one comparison. This is **not** sufficient to keep or remove semantic.

## 3. Required experiment (not run)

Compare original / lexical-only V7 / lexical V7 + deterministic bridge / conditional
semantic V7 on **fresh** development tasks (direct, two-hop, synonyms, aliases, long
context, contradictions, missing info, malicious memories), measuring correctness,
selected evidence, latency, tokens, and memory. This is **NOT TESTED** in Phase 5.

## 4. Decision

- Semantic/ANN/neural remain **optional and off by default** (unchanged).
- No permanent semantic removal is made without fresh evidence, per the mission.
- A deterministic lexical bridge (Phase B's `slotMatch`/`linked` machinery already
  provides a deterministic two-hop bridge) is the candidate to compare against semantic.

## 5. Verdict

**NOT TESTED.** Default path is lexical-first (PASS on the "simple by default" goal);
whether conditional semantic can be removed requires the fresh ablation above, which is
deferred and must not be replaced by the historical pilot.
