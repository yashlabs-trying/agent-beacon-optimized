# Phase 5 — Contradiction and Ambiguity

## Contradiction (B1, 20 tasks)

| Arm | A | B | C | D | E |
|---|---:|---:|---:|---:|---:|
| B1 | 0/20 | 8/20 | 8/20 | 8/20 | 8/20 |

- Half the tasks are direct correction (old→new value): all memory arms resolve these.
- Half are genuinely unresolvable (two equally credible notes): the correct behavior is
  abstention. The model abstains in some, guesses in others.
- **All arms including the oracle score 8/20 → contradiction resolution is a model
  reasoning ceiling**, not a memory failure. The memory system delivers the conflicting
  evidence; the model fails to adjudicate.
- Detection (locked Phase 3 set, reused as regression) remains precision 1.000 / recall
  0.500 default, cross-kind opt-in 1.000. Target 95/95 is **NOT MET** at default.

## Ambiguity (B2, 20 tasks)

| Arm | A | B | C | D | E |
|---|---:|---:|---:|---:|---:|
| B2 | 10/20 | 20/20 | 16/20 | 16/20 | 20/20 |

- Resolvable cases (default deploy env): memory arms answer correctly.
- Unresolvable cases (deploy now, no target): the correct behavior is abstention.
  **Original Beacon (lexical) abstains correctly more often (20/20) than the semantic
  arms (16/20)** — because lexical retrieves less, so the model has no hint to over-answer
  from. The intelligent layer's sufficiency signal helps but does not fully close the gap
  on 4 tasks.
- **This is a regression of semantic/intelligent relative to original on ambiguity.**
- Excessive abstention was not observed (memory arms still answer resolvable cases).

## Answers

- Does V6 resolve contradictions better? **EQUIVALENT** to original (both 8/20; model ceiling).
- Does V6 handle ambiguity better? **WORSE** than original (16 vs 20) — semantic over-answers.
- Target ≥95% contradiction P/R and ≥95% ambiguity: **NOT MET** (report denominators above).
