# Phase 5 — Agent Intelligence (Suite B) and Overall Task Success

## Overall (300 tasks, seed 0)

| Arm | Success | Acc |
|---|---:|---:|
| A none | 30/300 | 0.100 |
| B original | 229/300 | 0.763 |
| C semantic | 209/300 | 0.697 |
| D intelligent | 224/300 | 0.747 |
| E oracle | 246/300 | 0.820 |

Paired: **D vs B 17–22–261, p=0.52 (tied, B marginally ahead)**; D vs C 15–0–285, p=0.0001.

## Suite B (intelligence), per category

| Category | A | B | C | D | E |
|---|---:|---:|---:|---:|---:|
| B1 contradiction | 0/20 | 8/20 | 8/20 | 8/20 | 8/20 |
| B2 ambiguity | 10/20 | 20/20 | 16/20 | 16/20 | 20/20 |
| B3 multi-hop | 0/20 | 0/20 | 8/20 | 8/20 | 7/20 |
| B4 decision | 0/20 | 20/20 | 20/20 | 20/20 | 20/20 |
| B5 trust/adversarial | 0/20 | 14/20 | 5/20 | 20/20 | 20/20 |
| **Suite B total** | 10/100 | 62/100 | 57/100 | 72/100 | 75/100 |

## Findings

1. **D is the best memory configuration on Suite B (72/100)**, ahead of B (62) and C (57).
   Its win is almost entirely **B5 trust/adversarial (20/20 vs B 14, C 5)**.
2. **Semantic retrieval hurts B2 ambiguity** (B 20 vs C/D 16): the extra retrieved hint
   pushes the model to over-answer on unresolvable cases (the same defect Phase 4D fixed
   on the dev set, but here 4 tasks still fail).
3. **Semantic retrieval helps B3 multi-hop** (B 0 vs C/D 8): lexical cannot retrieve the
   two-hop evidence chain; semantic can. But even oracle only reaches 7/20 → the residual
   is a **model reasoning** failure, not memory.
4. **B1 contradiction is a model ceiling** (all arms 8/20, including oracle): the model
   fails to resolve contradictions even with the correct evidence.
5. **B4 decision is saturated** (all memory arms 20/20).

## Interpretation

Memory helps a great deal (B vs A 199–0–101). Among memory systems, the differences are
concentrated: semantic recall helps multi-hop but hurts precision/ambiguity; the intelligent
layer's net effect is to fix the security sub-category, landing Suite B ahead of both
baselines. Across all 300 tasks, however, semantic's precision losses elsewhere (A3)
cancel that gain, so **D ties B overall**.

## Verdict

- V6 intelligent vs original (overall task success): **EQUIVALENT** (p=0.52).
- V6 intelligent vs V6 semantic: **BETTER** (p=0.0001).
- Memory vs no memory: **BETTER** (p<1e-4).
