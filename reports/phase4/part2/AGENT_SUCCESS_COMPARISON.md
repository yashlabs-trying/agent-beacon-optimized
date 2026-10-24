# Phase 4B — Agent Success Comparison (Development Pilot)

**Question: does V6 beat original Beacon in actual agent task success?**

On the development adversarial pilot: **yes, decisively — but the win comes from the
opt-in semantic layer, not the base engine.**

## Headline

| Comparison | Result | Paired (W/L/T) | p |
|---|---|---|---|
| V6 semantic (hashed/neural) vs original Beacon | **+65–70 pts** | 13–14 / 0 / 6–7 | ≤0.0002 |
| V6 neural vs no memory | +70 pts | 14 / 0 / 6 | 0.0001 |
| V6 neural vs V6 hashed | +5 pts (not significant) | 1 / 0 / 19 | 1.00 |
| V6 lexical vs original | identical | 0 / 0 / 20 | 1.00 |

Accuracy: none 0.10, original 0.10, v6_lexical 0.10, v6_hashed 0.75, v6_hybrid 0.75,
v6_neural 0.80, oracle 0.95 (n=20).

## The essential caveat

`original` and `v6_lexical` both score 0.10 because the **shipped lexical path returns no
memories for natural-language queries** (AND-substring predicate; verified by running the
real CLI: `memory list --query "Which database is Project Atlas currently running?"`
→ `[]` for both `beacon-original` and `beacon-v6`). So the +65–70 pt effect is:

> **V6's opt-in semantic retrieval vs the lexical default of either system.**

This is a real and important V6 capability — original Beacon has no semantic path at all —
but it is **not** "the V6 engine is better than the original engine"; the shared lexical
engine is byte-identical.

## Does retrieval improvement translate to task success?

Yes on this split: required-evidence recall rose 0.10 → 1.00 and success rose
0.10 → 0.80. But the residual gap to oracle (0.80 → 0.95) is **reasoning**, not retrieval:
even with the exact gold evidence, the model fails the extreme historical-temporal case.

## Fairness

- Same model, prompt, task text, K, and grading for every arm.
- Retrieval differs by design (that is the thing under test); external resources and
  output budget are comparable (K=5, num_predict=256).
- Oracle receives only the gold required-evidence, never the answer or future facts.
- Original/V6 lexical arms executed the real CLI binaries.

## Verdict for this comparison

**PASS (with scope caveat):** V6's semantic retrieval substantially and significantly
improves agent task success over the lexical default. **NOT a claim** that V6's base
engine beats original Beacon, nor a held-out generalization (single seed, dev split).
