# Retrieval quality report

Status: lexical measured; semantic (hashed) measured; neural **not implemented**;
ANN **rejected**; ranking analysis measured.

## Methods compared (measured, 17-case labeled held-out set)

| method | recall@1 | recall@5 | MRR | nDCG@5 |
|---|---|---|---|---|
| lexical (v2, original Beacon search) | 0.118 | 0.412 | 0.235 | 0.280 |
| semantic-only (hashed) | 0.412 | 0.706 | 0.549 | 0.590 |
| hybrid RRF | 0.294 | 0.706 | 0.490 | 0.546 |

Per category (recall@1 / recall@5):

| method | overlap | exact | synonym | misleading |
|---|---|---|---|---|
| lexical | 0.25 / 0.50 | 0.00 / 1.00 | 0.00 / 0.00 | 0.00 / 0.00 |
| semantic | 0.75 / 1.00 | 0.33 / 1.00 | 0.00 / 0.33 | 0.00 / 0.00 |
| hybrid | 0.50 / 1.00 | 0.33 / 1.00 | 0.00 / 0.33 | 0.00 / 0.00 |

## Findings

- Semantic retrieval improves recall@5 from 0.41 to 0.71 on this set, entirely
  from the `overlap` and `exact` categories.
- **Hybrid RRF underperforms semantic-only on recall@1 (0.29 vs 0.41).** Cause:
  RRF sums 1/(60+rank) from both sources; a strong semantic rank-1 hit scores
  1/61 from semantic but is diluted by a weak lexical contribution, while a
  memory ranked highly by both accumulates more. On this set the lexical source
  is weaker, so RRF's equal weighting pulls the ranking down. A weighted RRF
  (lexical weight < semantic weight) is the obvious fix and was **not**
  implemented (no held-out tuning set large enough to avoid overfitting).
- **`synonym` is zero for every method.** The hashed embedder is lexical; true
  synonymy needs a neural model, which is not implemented. This is the single
  biggest quality gap.
- **`misleading` is zero for every method.** A near-verbatim wrong memory
  outranks the correct paraphrase; ranking cannot fix a flat lexical signal.

## Neural embedding comparison

**Not performed.** No offline, redistributable, pure-Go neural embedding backend
could be integrated here (see OPEN_SOURCE_LIBRARY_AUDIT.md). Reporting numbers
for it would be fabrication.

## Failure cases

- Morphological overlap works ("expiry"/"expired") but synonymy does not.
- Identifier queries (symbols, paths, error strings) are handled best by the
  lexical path; the semantic path adds little and can add noise, which is why
  lexical remains the default.

## Held-out discipline

The 17 cases were authored for evaluation; the hashed embedder has no training
step, so there is no train/test contamination. The set is small; confidence
intervals are wide and not reported.
