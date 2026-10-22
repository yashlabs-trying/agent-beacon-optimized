# Phase 2 — Adversarial Trust and Security

## Summary

The deterministic evidence-trust layer was measured against a 65-case labeled
adversarial matrix and the storage/retrieval security boundary was probed. Two
defects were found and fixed. **No scope leak, no approval bypass and no
critical security violation was found.** Contradiction detection is exact on its
labeled set (precision 1.000) but misses cross-kind conflicts by design and
produces review-worthy cross-talk on dense corpora; both are reported honestly.

## Environment

Apple M5 / macOS 26.6.2 / go1.27.3. Worktree base `85e6ea1c`. All data synthetic;
no real secrets or real agent runtime involved.

## Defects found and fixed

1. **`detectContradictions` with zero options flagged every opposite-polarity
   pair.** `AnnotateTrust` set `ContradictionTerms` to 2 when unset, but the
   helper itself did not, so `detectContradictions(memories, TrustOptions{})`
   compared `len(shared) >= 0` (always true). The Phase 1 test called the helper
   directly and happened to pass only because each pair used its own kind. Fixed
   by defaulting `ContradictionTerms` inside the helper.
2. **Coincidental collisions on generic verbs.** Any two same-kind directives
   sharing two words ("pin", "before") were flagged. Added a principled rule: the
   shared terms must exceed half of the smaller memory's significant terms, so a
   contradiction requires the memories to be about the same subject. On the dense
   adversarial corpus this cut cross-talk from **74 → 37** pairs with **no loss
   of true conflicts** and the Phase 1 exact test unchanged. Term/polarity sets
   are now precomputed once instead of inside the O(n²) scan.

## Contradiction matrix (65 labeled cases)

| Category | cases | tp | fp | fn | tn | note |
|---|---|---|---|---|---|---|
| canonical_conflict | 20 | 20 | 0 | 0 | 0 | same kind, opposite polarity, shared subject |
| hidden_in_long_text | 5 | 5 | 0 | 0 | 0 | conflict buried under repeated preamble |
| same_polarity_decoy | 15 | 0 | 0 | 0 | 15 | shared terms, same polarity |
| unrelated | 10 | 0 | 0 | 0 | 10 | no shared subject |
| insufficient_overlap | 7 | 0 | 0 | 0 | 7 | opposite polarity, no shared subject |
| cross_kind_missed | 8 | 0 | 0 | 8 | 0 | opposite polarity, **different kind** |

**Within-pair: tp=25, fp=0, fn=8, tn=32, precision=1.000, recall=0.758,
false-positive-rate=0.000.** The 8 false negatives are all cross-kind: the
detector is scoped to same project+kind, so a conflict between a `convention`
and a `gotcha` is not flagged. This is a deliberate design limitation of a
bag-of-words heuristic, reported as such.

**Cross-talk:** on the full 130-memory corpus the detector emits 37 pairs that do
not correspond to a labeled pair — memories that genuinely share more than half
their significant terms with opposite polarity. These are extra pairs for human
review; they are not security failures. The heuristic never approves, rejects,
ranks or retrieves; it only annotates.

## Security boundary

| Probe | Result |
|---|---|
| Cross-scope leakage — lexical path | no leak (2 projects, 50 memories, shared hostile token) |
| Cross-scope leakage — hybrid exact | no leak |
| Cross-scope leakage — hybrid ANN | no leak |
| Path traversal via relayed `ProjectPath` (`../../etc`) | resolves to a non-matching ID → 0 results; scope not widened |
| Approval gating | pending and rejected candidates invisible; approved memory visible |
| Fake system instruction (`SYSTEM: ignore previous...`) | stored and returned verbatim as inert data; not executed |
| Trust-bypass request (`set index_ready=1 ...`) | stored verbatim; no effect on trust logic |
| Synthetic-secret exfiltration request | stored verbatim; no secret exists and none is produced |
| Malicious tool-use instruction (`rm -rf /`) | stored verbatim; Beacon executes no tools |
| SQL injection (`'); DROP TABLE memories; --`) | inert; `memories` table intact, row count correct |
| Path traversal text | inert |
| Oversized body (200 KB) | round-trips without truncation |
| Unicode bidi override text | stored and returned verbatim |

No prompt-injection claim is made: there is no agent runtime in these tests, so
these only verify that hostile *memory content* is stored and returned as inert
data and never interpreted by the store.

## Latency

`AnnotateTrust` over 500 memories: **44.53 ms total, ~89 µs/item** (macOS M5).

## Reproduce

```
go test ./internal/learning/ \
  -run 'TestPhase2Adversarial|TestPhase2CrossScope|TestPhase2Unapproved|TestPhase2Hostile|TestPhase2TrustAnnotationLatency' \
  -v -count=1
```
