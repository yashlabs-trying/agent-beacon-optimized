# Phase 4B — Final Verdict

Development adversarial pilot: 20 cases × 7 arms, qwen2.5-coder:7b, deterministic
grading. Single seed, development split. Raw evidence in `RAW_EVIDENCE/`.

## Gates

| Gate | Status |
|---|---|
| Provenance verified (99c70908, bcf3a1fd, e032ee73) | **PASS** |
| Phase 3 locked fixtures preserved | **PASS** (`LOCK OK ×3`) |
| Harness audit | **PASS** |
| Dev corpus built + independently validated | **PASS** (20 cases, sha256 `05853A87…`) |
| Calibration pilot executed | **PASS** |
| Held-out 1,200-case comparison | **NOT RUN** |
| Scale 1K/10K/100K | **PASS** |
| Scale 1M | **BLOCKED** (RAM/time) |
| Concurrency ≤8 | **PASS** |
| Concurrency 16 / 32 / 64 | **FAIL / BLOCKED** |
| Fault injection / recovery | **PASS** |
| Context-budget matrix | **BLOCKED** |
| Adversarial injection safety | **FAIL** (dev) |
| Ambiguity discrimination | **BLOCKED** (non-discriminating dev set) |
| Published benchmarks (LongMemEval/LoCoMo/MemoryAgentBench) | **NOT RUN** (no local dataset; download not authorized) |

## The ten questions

1. **Does V6 beat original Beacon in actual agent task success?**
   On this dev split, **yes for the semantic layer** (+65–70 pts, p ≤ 0.0002), but
   **no for the base engine** — `v6_lexical` equals `original` exactly (0–0–20). The win
   is V6's opt-in semantic retrieval over the shared lexical default.

2. **In which categories does it win?**
   multi_hop, selection, distractor, context, generalization, evidence, current-temporal
   — all where semantic retrieval finds the fact and the lexical path finds nothing.

3. **Where does original Beacon remain better?**
   Nowhere on this split. But original is *equal*, not worse, on the two categories both
   systems pass (ambiguity) and on the base lexical path. No category shows original
   strictly better.

4. **How well does each handle contradictions and ambiguity?**
   Contradiction: semantic 1–2/4, oracle 4/4 — **weak**, limited by context
   contamination, not recall. Ambiguity: **undetermined** (dev cases non-discriminating).

5. **Which system is more reliable under concurrency and crashes?**
   Equal — they share the storage engine. Crash/recovery PASS; ≤8 clients PASS; 16-client
   p99 FAIL for both.

6. **Which system performs better under tight context budgets?**
   **Undetermined** (matrix BLOCKED). The one context case passed for all semantic arms.

7. **Does neural retrieval justify its extra latency and operating cost?**
   **Not yet.** Neural ≈ hashed on this dev set (1–0–19, p=1.0) while requiring a model,
   a sidecar, and ~43 ms/query over the LAN. No evidence yet justifies the cost.

8. **What causes the most important V6 failures?**
   **Trust-aware context selection.** Retrieval recall is 1.0; failures are (a) the agent
   obeying an injected memory, and (b) the agent abstaining when conflicting/stale
   memories are co-retrieved with the gold. Not retrieval misses.

9. **Which improvements are worth implementing next?**
   (1) Trust-tiered context assembly (withhold/label untrusted memory); (2) supersession-
   aware selection that collapses stale chains to the head before prompting; (3) pass
   provenance/timestamps so the model can adjudicate. Only after these, consider neural.

10. **Is V6 ready for external publication or production adoption?**
    **No.** The dev pilot shows a real, large semantic-retrieval win, but the held-out
    study is not run, the injection safety gate fails, ambiguity/context/scale-at-1M are
    unverified, and neural shows no advantage over hashed. Publish the *method and the
    negative findings*; do not claim production readiness.

## What would change the verdict

- A held-out ≥1,200-case run showing semantic V6 ≥ original +10 pts with a CI excluding
  0, **and** the injection gate passing, **and** a demonstration that neural beats hashed
  enough to justify its cost.

## One-line verdict

**V6's semantic retrieval is a genuine, large improvement over the lexical default on
hard memory tasks — but the remaining failures are context-selection and trust failures,
not retrieval failures, and V6 is not yet production- or publication-ready.**
