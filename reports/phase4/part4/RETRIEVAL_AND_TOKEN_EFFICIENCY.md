# Phase 4D — Retrieval and Token Efficiency

## 1. Retrieval routing

Phase 4A–4C evidence: on the locked 48-query fixture, hashed R@5 = 0.875, hybrid = 0.875,
neural = 0.979, lexical = 0.167; independent FTS5 = 0.917. Neural's quality edge over
hashed did **not** translate into a task-success edge in Phase 4B/4C (neural ≈ hashed),
while neural costs a sidecar and ~43 ms/query.

**Decision (measured, not assumed):** no adaptive routing heuristic was added. The
evidence does not justify the complexity, and the brief warns against routing heuristics
that depend on fixture answers. Retrieval modes remain available for ablation:
`--semantic` (offline hashed) is wired into the CLI; neural remains API/sidecar-only.

| Mode | R@5 (locked) | Cost | Used by |
|---|---:|---|---|
| lexical | 0.167 | ~0.5 ms, no model | default CLI |
| hashed | 0.875 | ~1 ms, offline | `--semantic` |
| hybrid | 0.875 | ~1 ms | API |
| neural | 0.979 | sidecar + ~43 ms | API/sidecar |

## 2. Token efficiency

| Arm (dev) | prompt tokens | accuracy |
|---|---:|---:|
| v6_semantic | 98 | 20/35 |
| **v6_intelligent** | **93** | **27/35** |
| oracle | 85 | 27/35 |

The intelligent layer uses **fewer** tokens than raw semantic retrieval (drops
stale/untrusted memories) and achieves higher accuracy — success per token improves. It is
not token-minimisation for its own sake: required-evidence recall stays 1.000.

## 3. Budget safety (Invariant G)

- Budget applies **after** supersession/conflict, so it cannot resurrect a superseded
  fact or drop a dispute signal (`TestContract_BudgetDoesNotResurrectSuperseded`,
  `TestContract_BudgetKeepsConflictSignal`).
- The sufficiency signal returns an explicit insufficiency result when the evidence does
  not cover the query, instead of silently omitting critical facts.

## 4. Not run (honest)

- The multi-budget sweep (256/512/1024/2048 tokens) with the trust layer composed with
  the token-budget selector is **BLOCKED** (compute). The two selectors are not yet
  composed in production.
- Per-token cost in monetary units is **NOT MEASURED** (local model).

## Verdict

Token efficiency: **PASS** (higher accuracy at fewer tokens). Retrieval routing:
**NOT ADDED** (evidence did not justify it). Budget matrix: **BLOCKED**.
