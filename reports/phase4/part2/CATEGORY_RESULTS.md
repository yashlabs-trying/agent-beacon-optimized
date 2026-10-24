# Phase 4B — Category Results (Development Pilot)

Per-category correctness, 20 cases × 7 arms. Development split only.

| Arm | adv | amb | ctx | contra | distr | evid | gen | mhop | sel | temp | overall |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| none | 0/2 | 2/2 | 0/1 | 0/4 | 0/1 | 0/1 | 0/2 | 0/2 | 0/2 | 0/3 | 2/20 |
| original | 0/2 | 2/2 | 0/1 | 0/4 | 0/1 | 0/1 | 0/2 | 0/2 | 0/2 | 0/3 | 2/20 |
| v6_lexical | 0/2 | 2/2 | 0/1 | 0/4 | 0/1 | 0/1 | 0/2 | 0/2 | 0/2 | 0/3 | 2/20 |
| v6_hashed | 1/2 | 2/2 | 1/1 | 1/4 | 1/1 | 1/1 | 2/2 | 2/2 | 2/2 | 2/3 | 15/20 |
| v6_hybrid | 1/2 | 2/2 | 1/1 | 1/4 | 1/1 | 1/1 | 2/2 | 2/2 | 2/2 | 2/3 | 15/20 |
| v6_neural | 1/2 | 2/2 | 1/1 | 2/4 | 1/1 | 1/1 | 2/2 | 2/2 | 2/2 | 2/3 | 16/20 |
| oracle | 2/2 | 2/2 | 1/1 | 4/4 | 1/1 | 1/1 | 2/2 | 2/2 | 2/2 | 2/3 | 19/20 |

## Category notes

- **Contradiction (hardest).** Semantic arms 1–2/4; oracle 4/4. The gold evidence is
  retrieved (recall 1.0) but conflicting distractors are retrieved alongside it and the
  model abstains or picks wrong. Retrieval precision/selection, not recall, is the gap.
- **Temporal.** 2/3 for semantic arms; the historical-version case defeats even the
  oracle (pure reasoning failure). Current-version case fails when both versions are in
  context.
- **Multi-hop / selection / distractor / context / generalization.** Semantic arms solve
  these (2/2 or 1/1); lexical arms score zero because they retrieve nothing.
- **Adversarial.** Semantic arms 1/2: they surface the poisoned note and the model obeys
  it. Oracle (gold only) is 2/2.
- **Ambiguity.** All arms 2/2 — non-discriminating here (no-memory also abstains).
- **Evidence.** v6_hashed/neural/hybrid 1/1 (answer + source); lexical 0/1.

## Non-discriminating categories (benchmark-design debt)

- `ambiguity` must be rebuilt for the held-out split so a memory-less agent cannot pass
  by default (e.g. pair a clear case with a near-miss so abstention is only correct when
  evidence is genuinely absent, and include "should NOT abstain" cases).
- `distractor`, `context`, `evidence` have only 1 case each — insufficient; expand in
  the held-out split.
