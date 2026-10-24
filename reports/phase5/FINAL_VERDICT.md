# Phase 5 — Final Verdict

No marketing language. Every claim links to measured evidence in `raw/` and the reports.

## 1. The twelve questions

1. **Is V6 more useful to a real agent than original Beacon?**
   **No measurable advantage.** V6 intelligent 224/300 vs original 229/300 (paired 17–22,
   p=0.52). VERDICT: **EQUIVALENT (marginally worse)**.

2. **How large is the improvement in end-to-end success?**
   **−1.7 points** (D 224 vs B 229), not significant. No improvement.

3. **Does intelligent context selection outperform semantic retrieval alone?**
   **Yes, significantly:** D 224 vs C 209 (15–0–285, p=0.0001). VERDICT: **BETTER**.

4. **Does memory help agents avoid repeating mistakes?**
   **Not established.** A2 supersession memory-positive (19/20 vs 0/20) but equal across
   memory systems; A1 (learning from failure) is **invalid** (oracle 0/20). VERDICT:
   **INCONCLUSIVE**.

5. **Does the system resolve contradictions better?**
   **No.** B1: all memory arms 8/20 including oracle. Detection P=1.0/R=0.50 default.
   VERDICT: **EQUIVALENT / FAIL** (model ceiling).

6. **Can it distinguish uncertainty from factual knowledge?**
   **Partially, and worse than original.** B2 ambiguity: original 20/20 vs
   semantic/intelligent 16/20 — semantic over-answers. VERDICT: **WORSE**.

7. **Does it recover from supported failures without corrupting data?**
   **Yes at the system level** (crash/durability suites PASS, 0 acknowledged-write loss);
   agent-level A4 equal across memory arms (14/20). VERDICT: **EQUIVALENT/PASS**.

8. **Does it maintain correctness under large histories and high concurrency?**
   Selector near-linear (36× at n=1000); 10/50/100-session history PASS; ≤8 clients PASS,
   16 clients FAIL; 1M and 32/64 **NOT TESTED**. VERDICT: **CONDITIONAL**.

9. **How much real agent latency and cost does it save?**
   **Latency: yes** (D 0.92 s vs B 1.18 s, −22%); **tokens: no** (149 vs 131). Time per
   success: D 1.23 s vs B 1.55 s. VERDICT: **BETTER latency, WORSE tokens**.

10. **Strongest counterexamples against V6?**
    - **A3 knowledge transfer: original 20/20 vs V6 2/20** — semantic over-retrieval
      derails the model on a simple fact lookup (18-task swing).
    - **B2 ambiguity: original 20/20 vs V6 16/20.**
    - **B5 under semantic-only: 5/20 vs original 14/20** (security regression; fixed by
      the intelligent layer).

11. **Which changes produced real downstream gains vs only microbenchmarks?**
    - Real gains: the **trust/injection layer** (B5 20/20 vs semantic 5/20), and **semantic
      recall for multi-hop** (B3 0→8/20).
    - Microbenchmark only: the **36× selector speedup** — invisible end-to-end (K=8).
    - Negative: **semantic retrieval's extra context** hurt precision/ambiguity.

12. **What must be solved next to be substantially better?**
    Question-relevance filtering of context (A3/B2) and contradiction/multi-hop reasoning
    (B1/B3) — see §4.

## 2. Evidence table

| Claim | Classification |
|---|---|
| Memory >> no memory (199–0–101, p<1e-4) | **PROVEN** |
| V6 intelligent > V6 semantic (p=0.0001) | **PROVEN** |
| V6 semantic < original (p=0.0029) | **PROVEN** |
| V6 intelligent fixes the B5 security regression (20/20 vs 5/20) | **PROVEN** |
| V6 intelligent ≈ original overall (p=0.52) | **PROVEN (no advantage)** |
| Semantic helps multi-hop (0→8/20) | **SUPPORTED BUT LIMITED** (oracle 7/20) |
| Selector 36× faster at n=1000 | **SUPPORTED BUT LIMITED** (no agent-level effect) |
| D lower latency, higher tokens | **SUPPORTED BUT LIMITED** |
| V6 ≥ original +10 pts | **FAILED** |
| Contradiction ≥95/95 | **FAILED** (default recall 0.50; agent 8/20) |
| Ambiguity ≥95% | **FAILED** (16/20; worse than original) |
| Learning from feedback (A1) | **FAILED (invalid task)** |
| V6 superiority on unseen tasks | **FAILED** |
| 1M scale, 32/64 clients, neural-in-loop, external benchmarks | **UNKNOWN (NOT TESTED)** |

## 3. Five highest-impact engineering improvements justified by observed failures

1. **Question-relevance context filtering** (fixes A3, B2). Filter retrieved memories by
   relevance to the *question*, not just the query, and prefer precision when the task is a
   fact lookup. Observed impact: up to +18 tasks on A3 and +4 on B2.
2. **Reasoning-support prompting for contradictions/multi-hop** (fixes B1 8/20, B3 7/20).
   Provide provenance/timestamps and an explicit resolution procedure; the memory system
   already delivers the evidence.
3. **Semantic supersession** (A2 currently lexical; paraphrases missed). Reduces reliance on
   correction keywords.
4. **A fairer, harder Suite C** (C2/C3/C4/C5 are saturated for memory arms; C3 doesn't test
   security). Replace with genuinely discriminating engineering/security tasks.
5. **Real provenance pipeline** so trust does not depend on benchmark `source:` tags.

## 4. What the experiments do and do not establish

- **Do establish:** memory helps; the intelligent layer beats semantic and repairs its
  security regression; the selector is fast; recovery/durability hold; V6 does **not**
  beat original overall; semantic hurts precision/ambiguity.
- **Do not establish:** any V6 advantage over original; seed-level robustness beyond
  determinism; 1M/high-concurrency behaviour; external-benchmark standing; tool-level
  security.

## 5. Verdict

**V6 intelligent is NOT more useful to a real agent than original Beacon on this
evaluation.** It is a significant improvement over V6 semantic and over the security
regression semantic introduced, with better latency but more tokens. It is a
**specialized** improvement (trust/security), not a general one. The benchmark is honest,
frozen, and reproducible; the losses are preserved, not hidden.
