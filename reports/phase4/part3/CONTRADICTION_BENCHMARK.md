# Phase 4C — Contradiction Benchmark

Eleven development contradiction cases spanning the brief's ten sub-types, plus a
non-conflict control. Detection and resolution are reported **separately**.

## 1. Per-case outcome (dev)

| Case | sub-type | semantic | +trust | +super | +conflict | +inject | **intelligent** | oracle |
|---|---|---|---|---|---|---|---|---|
| cx_direct_correction | direct | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| cx_false_newer | false newer | ✗ | ✓ | ✗ | ✗ | ✗ | ✓ | ✓ |
| cx_project_specific | project | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| cx_version_specific | version | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| cx_temporal_obsolete | temporal | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| cx_cross_kind | cross-kind | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| cx_negation | negation | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| cx_unresolvable | unresolvable | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| cx_multihop | multi-hop | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| cx_adversarial_correction | adversarial | ✗ | ✓ | ✗ | ✗ | ✗ | ✓ | ✓ |
| cx_false_positive_control | non-conflict | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| **Total** | | 5/11 | 7/11 | 5/11 | 5/11 | 5/11 | **7/11** | 7/11 |

## 2. Detection vs resolution (locked Phase 3 set)

- **Detection** (locked 24 pairs): precision **1.000**, recall **0.500** default;
  opt-in cross-kind recall **1.000** at precision 1.000. Same-kind recall 1.000.
- **Resolution** (dev): the layer fixes `cx_false_newer` and `cx_adversarial_correction`
  by rejecting untrusted corrections, and `cx_multihop` by superseding the stale port.

## 3. Where resolution still fails (and why)

| Case | Failure class | Note |
|---|---|---|
| cx_version_specific | agent reasoning | both versions legitimately apply to different scopes; the model must scope, and fails even with oracle |
| cx_temporal_obsolete | lexical supersession miss → reasoning | paraphrase ("workaround" vs "no longer needed") with low lexical overlap |
| cx_cross_kind | unresolvable-by-design | both credible; correct is uncertainty; model guesses |
| cx_negation | unresolvable-by-design | both credible; model guesses |
| cx_unresolvable | agent reasoning | oracle abstains; intelligent answers a wrong value (over-answer defect) |

## 4. False supersession and false rejection

- False supersession was found and **fixed** during Stage C: a "which service uses which
  database" note was wrongly superseded by a port correction sharing only the database
  name. Fix: require shared subject terms ≥ half of the smaller memory's terms.
- False rejection: trust-only drops `adv_mixed` (a useful fact bundled with an injected
  instruction); the combined layer sanitizes and keeps it. Reported, not hidden.

## 5. Non-contradictory control

`cx_false_positive_control` (cache TTL 60 vs session TTL 900) is **not** flagged as a
conflict by the layer and is answered correctly — the detector does not simply flag
everything.

## Verdict

Detection: **PASS** (locked). Resolution: **CONDITIONAL** — improved by trust/supersession
but limited by (a) lexical paraphrase misses and (b) model reasoning on genuinely
unresolvable conflicts.
