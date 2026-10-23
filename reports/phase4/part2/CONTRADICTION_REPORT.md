# Phase 4B — Contradiction Report (Development Pilot)

4 contradiction cases (hard/extreme). Gold evidence is retrieved in every semantic-arm
run (recall 1.0); the failures are in **resolution**, not retrieval.

## Results

| Case | none | original | v6_lexical | v6_hashed | v6_hybrid | v6_neural | oracle |
|---|---|---|---|---|---|---|---|
| contra_atlas_db_current | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ | ✓ |
| contra_atlas_db_approved | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ |
| contra_auth_scheme | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ |
| contra_ratelimit | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ |
| **Total** | 0/4 | 0/4 | 0/4 | 1/4 | 1/4 | **2/4** | **4/4** |

## Diagnosis

- **Retrieval is not the problem.** For `contra_atlas_db_current`, v6_hashed retrieved
  `[day1, day15, day12, day3, day9]` — the gold Day-12 verified inspection **is** present
  (rank 3). For `contra_atlas_db_approved`, v6_neural retrieved the gold `atlas2-db-day5`
  at rank 1. Yet both systems answered "Cannot determine…".
- **Context contamination is the problem.** When the context contains the current fact,
  the stale proposal, the canceled migration, and an untrusted document together, the
  model refuses to adjudicate. The **oracle** — which sees *only* the gold note — answers
  correctly every time (4/4).
- **Failure class:** agent reasoning / evidence selection, **not** memory retrieval.
  This is the single most important architectural finding of Phase 4B: V6's recall is
  excellent, but its **precision-at-context** is not, and the model cannot recover from
  conflicting memories in-context.

## What would fix it (candidates, not implemented)

1. Rank/supersede stale and untrusted memories out of the context (trust-aware context
   assembly), so the model sees the minimal consistent evidence.
2. Pass provenance/trust labels and timestamps to the prompt so the model can adjudicate.
3. Contradiction-aware selection that collapses a superseded chain to its head.

## Gate

Contradiction recall/precision on the Phase 3 locked set remain P=1.000 / R=0.500
(`SECURITY_RELIABILITY_RESULTS.md` of Phase 4A). The Phase 4B dev contradiction cases
show the **agent-level** consequence: even with correct retrieval, resolution fails under
conflicting context. Held-out contradiction evaluation: **NOT RUN**.
