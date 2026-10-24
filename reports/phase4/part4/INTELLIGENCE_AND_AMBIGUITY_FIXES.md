# Phase 4D — Intelligence and Ambiguity Fixes

## 1. The Phase 4C defect

On unresolvable ambiguity the memory arms over-answered: a plausible but insufficient
retrieved hint (`amb_hint`) pushed the model to name a wrong value where a memory-less
agent abstained.

## 2. The fix: a structured evidence-sufficiency signal

`assessSufficiency` computes coverage of the query's significant terms by the selected
evidence, with **light stemming** (terms match on a ≥4-char shared prefix, so
`timestamp`/`timestamps`, `store`/`stored`, `roll`/`rollouts` match). If coverage < 0.3,
the result is marked `Insufficient` with a reason; with `WithholdIfInsufficient` the
evidence is withheld so the agent does not guess. The signal distinguishes:

| Situation | Signal |
|---|---|
| Answer supported | `Sufficient=true`, note "evidence covers the query" |
| Missing info | `Insufficient=true`, note "query terms not covered…" |
| Conflicting evidence | `Sufficient=true` + `Unresolved=true` + `Uncertainty` |
| No applicable evidence after filtering | `Insufficient=true`, note "no applicable evidence" |
| Memory not needed | (no notes) — the control prompt must allow general knowledge (see §5) |

This does **not** always abstain (resolvable cases still answer) and does **not** simply
pick the top-ranked memory.

## 3. Iteration and the correctness gate

- First attempt (coverage threshold 0.4, no stemming): dev ambiguity fell 7/8 → 6/8 and
  overall 26 → 25/35 — **two regressions** (`am_implicit`, `am_intent`) from false
  insufficiency on stemmed words. Per the brief's rule, this was **not** kept as-is.
- Corrected (stemming + threshold 0.3): dev ambiguity **8/8**, overall **27/35**, and the
  unresolvable case now abstains correctly. No regressions.

## 4. New/updated ambiguity cases

Development (8): resolvable, unresolvable, misleading, competing conventions, implicit,
intent, high-risk, should-not-abstain. Held-out (4): resolvable, unresolvable, misleading,
should-answer. All memory-required cases discriminate (no-memory scores 3/8 and 1/4).

## 5. Controls (repair)

Phase 4C controls were invalid: the note-only system prompt made the model abstain on
memory-independent questions (0/4 and 0/2 for every arm, including oracle). **Fix
identified, not yet applied to the pilot:** controls require a prompt that permits general
knowledge (or a separate "no-memory-needed" arm instruction). Controls remain excluded from
the memory comparison and are flagged for Phase 5. This is a benchmark-harness fix, not a
memory-system change.

## 6. Measured result (dev, 35 cases)

| Arm | ambiguity | overall |
|---|---:|---:|
| none | 3/8 | 6/35 |
| v6_semantic | 7/8 | 20/35 |
| **v6_intelligent** | **8/8** | **27/35** |
| oracle | 8/8 | 27/35 |

## Verdict

Ambiguity handling: **PASS** (8/8 dev, unresolvable fixed, no over-abstention).
Controls: **CONDITIONAL** (repair designed, not applied).
