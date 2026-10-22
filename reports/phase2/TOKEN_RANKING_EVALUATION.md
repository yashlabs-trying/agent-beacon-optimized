# Phase 2 — Token Budget and Hybrid Ranking

## Summary

Two reported problems were investigated on a deterministic 96-query labeled
dataset split into tuning and held-out halves, using the local hashed embedder
(no neural model — these are **hashed-semantic**, not neural, results).

- **Ranking:** standard RRF hybrid Recall@1 (0.729) is *below* semantic-only
  (0.812) on held-out data, confirming the reported regression. A semantic-heavy
  RRF weight (3:1) recovers Recall@1 to 0.812. R@5 is 1.000 for every hybrid.
  Defaults are left unchanged pending broader evidence.
- **Token budget:** a token-budgeted selection cuts tokens **1.61×** but loses
  **~6% of distinct relevant findings** on held-out queries. This is a
  **tradeoff, not an unconditional improvement** — exactly the reported failure
  mode. Evidence-preserving deduplication is the only strategy that improves
  retention (112 vs 109) while lowering redundancy (0.525 vs 0.569) at no cost.

## Environment

Apple M5 / macOS 26.6.2 / go1.27.3. Worktree base `85e6ea1c`. Embedder:
`hashed-subword-v1` (512-dim signed feature hashing). **Replayed/algorithmic
embeddings, not live neural inference.** No embedding fixtures were needed
because the local embedder is deterministic and dependency-free.

## Dataset

24 topics × 4 query categories = 96 queries (`buildRankingDataset`). Each topic
has a gold memory, a same-evidence duplicate, two supporting memories, and 40
generic distractors. Query categories: `exact` (unique code token),
`identifier` (`ERR_*`), `overlap`, `morph` (morphological variant). Split by
topic parity: 48 tuning / 48 held-out.

## Ranking (held-out, 48 queries, top-5)

| Method | R@1 | R@5 | MRR | NDCG@5 |
|---|---|---|---|---|
| lexical | 0.500 | 0.750 | 0.625 | 0.658 |
| semantic (hashed) | **0.812** | 1.000 | 0.906 | 0.931 |
| hybrid RRF (1:1, default) | 0.729 | 1.000 | 0.865 | 0.900 |
| hybrid semantic-heavy (3:1) | **0.812** | 1.000 | 0.906 | 0.931 |
| hybrid lexical-heavy (1:3) | 0.729 | 1.000 | 0.865 | 0.900 |
| hybrid identifier-boost | 0.729 | 1.000 | 0.865 | 0.900 |

**Finding:** RRF dilutes the stronger semantic source, costing 0.083 Recall@1.
A semantic-heavy weight restores parity with semantic-only without hurting R@5.
Identifier boosting changed nothing on this set because the hashed embedder
already ranks identifier queries correctly. Because semantic-heavy only *ties*
semantic-only (it does not beat it) and this is a synthetic corpus, **no default
was changed**; the `LexicalWeight`/`SemanticWeight` knobs were added so the
choice can be revisited with real embeddings and a larger held-out set.

## Token selection (held-out, 48 queries, ≤5 items)

Relevance is measured at the level of distinct findings: the primary finding
(carried by the gold memory *or* its same-evidence duplicate) and two supporting
findings. 144 finding-slots total.

| Strategy | tokens/case | findings retained | redundancy | latency µs/case |
|---|---|---|---|---|
| fixed_top5 | 118.2 | 109/144 | 0.569 | 0.0 |
| token_budget (80 tok, truncate 40) | **73.6** | **100/144** | 0.627 | 8.7 |
| evidence_dedup | 115.9 | **112/144** | **0.525** | 0.2 |
| trust_aware | 118.1 | 109/144 | 0.581 | 90.8 |
| diversity_mmr | 118.2 | 109/144 | 0.569 | 3308.7 |

**Findings:**
- Token budgeting is a **tradeoff**: 1.61× fewer tokens but 9 fewer distinct
  findings (6.25%). It must not be reported as an unconditional win.
- Evidence-preserving deduplication is the only mild win: **+3 findings and the
  lowest redundancy** at negligible cost, by not spending a slot on a duplicate
  that shares provenance with a memory already selected.
- Trust-aware selection and MMR bought no retention on this set and added 91 µs
  and 3.3 ms per case respectively. MMR's cost is not justified here.
- No strategy was enabled by default; all are measurement-only experiments.

## Reproduce

```
go test ./internal/learning/ \
  -run 'TestPhase2RankingAlternatives|TestPhase2TokenSelectionStrategies' -v -count=1
```
