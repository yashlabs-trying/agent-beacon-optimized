# Beacon V7 hardening — Phase I: Component Ablations

Status: **Mostly NOT TESTED in this pass.** A fresh model-backed ablation campaign was
not run; this report records the available evidence and what remains.

## 1. Configurations of interest

| ID | Configuration |
|---|---|
| Original | `99c70908` lexical baseline |
| V6 intelligent | `02e34f98` (Phase 5 frozen) |
| V7 lexical-only | `SearchQuestion` disabled → `ListMemories` |
| V7 question-aware lexical | `SearchQuestion` (no embedder) |
| V7 conditional semantic | `SearchQuestion` + hashed embedder |
| V7 + sufficiency | `SearchQuestion` sufficiency/withhold signals |
| Full V7 | all of the above |
| Security-disabled | research-only, never a default |

## 2. Available evidence

- **Phase 5 (V6 intelligent vs original, 300 tasks):** original 229/300 vs V6
  intelligent 224/300 (paired 17–22, p=0.52) — equivalent; V6 semantic 209 (worse).
- **V7 pilot (56 tasks, historical/dev):** original 40/56, first V7 51/56, post-feedback
  56/56, semantic-disabled 48/56. This is seven repeated mechanism families and is **not**
  fresh validation.
- **Unit-level:** V7's question-aware attribute resolution and injection rejection are
  covered by 12 `TestSearchQuestion*` tests; the trust/supersession layer by 12 contract
  tests.

## 3. Not tested (honest)

A fresh, model-backed ablation of {original, V6 intelligent, V7 lexical-only, V7
question-aware lexical, V7 conditional semantic, V7+sufficiency, full V7} on a
held-out suite with matched model/budget: **NOT RUN** in Phase I. This is the key gap;
the mission's final 500-task benchmark is the intended venue and must not be run here.

## 4. What is established

- V7's question-aware lexical path fixes a general question-understanding defect
  (Phase B) with no regression to the default path.
- V7's warm allocation overhead is reduced (Phase C).
- V7 inherits the V6 trust/supersession/sufficiency behavior (Phases 4C/4D/5), which
  Phase 5 showed beats V6 semantic but ties original overall.

## Verdict

Component ablations (fresh, model-backed, held-out): **NOT TESTED**. Unit-level
correctness of the V7 components: **PASS**.
