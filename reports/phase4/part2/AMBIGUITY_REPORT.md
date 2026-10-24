# Phase 4B — Ambiguity & Abstention Report (Development Pilot)

2 abstention cases (`amb_service_owner`, `amb_missing_license`).

## Results

| Arm | correct abstention |
|---|---|
| none | 2/2 |
| original | 2/2 |
| v6_lexical | 2/2 |
| v6_hashed / hybrid / neural | 2/2 |
| oracle | 2/2 |

## Diagnosis — this category is NOT discriminating

Every arm passes because a memory-less agent also abstains ("Cannot determine from the
provided notes."). The cases therefore cannot distinguish memory systems and must not be
counted toward a victory.

## Required fix for the held-out split

The abstention category must be rebuilt so abstention is only correct when evidence is
genuinely absent, and must include:

1. **Should-not-abstain** cases: the fact is present but phrased indirectly — a good
   system answers, a lazy one abstains.
2. **Near-miss** cases: a similar but non-matching fact exists — correct behavior is to
   abstain (or to answer with the near-miss clearly flagged), punishing both over- and
   under-refusal.
3. **False-premise** cases: the question assumes a fact that is false — correct behavior
   is to reject the premise.
4. Score **unnecessary refusal** (abstaining when evidence exists) as a distinct failure,
   not a pass.

## Gate

**BLOCKED / non-discriminating on the dev split.** No ambiguity victory can be claimed;
the category needs redesign before held-out use.
