# Phase 5 — Failure Analysis

Failure taxonomy applied to every run. The distinction "retrieval correct but answer
wrong" is enforced: a case is labelled a retrieval failure only if the required evidence
was missing from the context.

## 1. D_intelligent failures by category (seed 0)

| Category | D failures | Dominant cause |
|---|---:|---|
| A1 learning from failure | 15 | **invalid task** (oracle 0/20) — grading/gold mismatch |
| A3 knowledge transfer | 18 | **over-retrieval**: semantic adds an unrelated verified note; model derailed |
| B1 contradiction | 12 | **reasoning** (oracle also 8/20) |
| B3 multi-hop | 12 | **reasoning** (oracle 7/20) |
| C1 coding | 7 | **model coding error** (same code for all memory arms) |
| A4 recovery | 6 | **model slip** (correct note retrieved) |
| B2 ambiguity | 4 | **over-answer** on unresolvable cases |
| A2 / C5 | 1 each | model slip |

## 2. Retrieval vs reasoning vs model

- **Retrieval-missing failures: 0** for C/D/E (mean required-evidence recall 1.000).
- The dominant failures are **reasoning** (B1/B3) and **over-retrieval/context
  contamination** (A3/B2), plus one **invalid task** (A1) and **model coding errors** (C1).

## 3. Where original Beacon wins (verified)

1. **A3 knowledge transfer: B 20/20 vs C/D 2/20.** Minimal reproducible case: query
   "Go services"; gold note "All Go services use UTC timestamps"; distractor note
   "The X team uses tabs, not spaces." Lexical returns only the gold note → correct.
   Semantic returns both → the model answers the distractor. **Root cause: semantic
   retrieval's extra context is not relevance-filtered to the question.**
2. **B2 ambiguity: B 20/20 vs C/D 16/20.** Lexical retrieves less, so the model abstains
   correctly on unresolvable tasks; semantic surfaces a hint that induces a guess.
3. **Overall: B 229 vs D 224.**

## 4. Where V6 intelligent wins (verified)

1. **B5 trust/adversarial: D 20/20 vs B 14/20, C 5/20.** The intelligent layer removes the
   malicious note; semantic alone is worse than original.
2. **Latency: D 0.92 s vs B 1.18 s.**

## 5. Equal-performance explanations

- A2/A4/A5/C1/C2/C4/C5: all memory arms retrieve the same gold evidence and the model
  behaves identically → no configuration difference is possible.
- B1/B3: all arms (including oracle) fail the same way → model reasoning ceiling.

## 6. Minimal failing cases (for Phase 6)

- `a3-*`: over-retrieval of a verified but question-irrelevant note.
- `b2-*` (unresolvable): a plausible hint induces a guess.
- `b5-*` under arm C: injected note obeyed.

## Verdict

The most important failures are **not retrieval failures** but **context-selection
precision** (A3/B2) and **model reasoning** (B1/B3). The intelligent layer fixes security
but does not yet fix question-relevance filtering.
