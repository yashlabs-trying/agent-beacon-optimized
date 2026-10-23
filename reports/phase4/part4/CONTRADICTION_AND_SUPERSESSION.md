# Phase 4D — Contradiction and Supersession

## 1. Detection (unchanged semantics, faster)

Detection reuses the trusted Phase 3 detector. Phase 4D adds
`detectContradictionsIndexed`, which visits only candidate pairs (memories sharing ≥2
significant terms, found via an inverted index) and applies **the same predicates**
(project scope, cross-kind gating, explicit-version separation, shared-term majority,
opposing polarity/action/negation). It is **equality-tested** against the O(n²)
`detectContradictions` (`TestContract_IndexedEqualsNaiveDetector`, plus the scaling test
asserts identical counts at n=20/100/500/1000).

- Locked 24-pair set: precision **1.000**, recall **0.500** default; cross-kind opt-in
  recall **1.000**. Preserved as a regression check, not fresh test data.

## 2. Resolution: structured, evidence-backed supersession

`resolveSupersessions` decides which fact applies using **explicit relationships**, not
recency alone:

- The `SupersededBy` field is honoured directly.
- A lexical correction supersedes an older fact only when: same project; compatible
  version; the newer memory carries a correction marker ("now", "no longer", "replaced",
  "corrected", …); ≥2 shared significant terms and ≥ half of the smaller memory's terms;
  and the newer memory is **trusted** (an untrusted newer note never overrides verified
  knowledge).
- **Chains** A→B→C resolve deterministically (each older is superseded; the head survives).
- **Cycles** are detected and broken fail-safe: the explicit flags in the cycle are
  cleared so the conflict is preserved rather than silently resolved.

History is never erased: superseded memories stay in the input (Invariant D) and are
reported in `Signals.SupersededIDs`.

## 3. Metrics

| Metric | Value |
|---|---|
| Detection precision (locked) | 1.000 |
| Detection recall (locked, default) | 0.500 (same-kind 1.000; cross-kind 0.000 default, 1.000 opt-in) |
| Resolution correctness (dev, 11 contradiction cases) | semantic 5/11 → **intelligent 7/11** → oracle 7/11 |
| False supersession rate | 0 on dev after the subject-overlap fix (verified: `svc-db` no longer wrongly superseded) |
| Unnecessary conflict flags | 0 on the non-conflict control (`cx_false_positive_control`) |
| Scope violations | 0 (`TestContract_ScopeIsolation`) |

## 4. Remaining resolution failures (honest)

`cx_cross_kind`, `cx_negation`, `cx_version_specific`, `cx_temporal_obsolete` fail for
**all** arms including the oracle — model reasoning / genuinely unresolvable conflicts,
not memory failures. Cross-kind detection stays **opt-in** where kind separation is an
intentional scope boundary.

## Verdict

Detection: **PASS** (unchanged, now near-linear). Structured supersession: **PASS**
(chains, cycles, false-newer). Resolution: **CONDITIONAL** (improved; residual failures
are model reasoning).
