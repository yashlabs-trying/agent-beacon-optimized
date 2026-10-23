# Phase 4D — Remaining Risks and Next Steps

## 1. Resolved in Phase 4D

- O(n²) selector → near-linear (31.5× at n=1000), identical results.
- Unresolvable-ambiguity over-answering → fixed (dev ambiguity 8/8).
- Weak injection coverage → encoded/zero-width detection added.
- Lexical-only supersession → structured `SupersededBy` + chains + cycle break.
- Layer unreachable in production → opt-in CLI `--semantic`/`--intelligent`.
- Un-tested interactions → 12 cross-component contract tests.

## 2. Open risks (with status)

| Risk | Severity | Status | Mitigation path |
|---|---|---|---|
| Model reasoning ceiling (cross-kind/negation/version cases fail for all arms incl. oracle) | High | **Open** | stronger model; per-scope reasoning prompts |
| Held-out superiority not significant (n=18, p=0.125) | High | **Open** | Phase 5 (300 tasks, 3 seeds) |
| 16-client write p99 (744–795 ms) | Medium | **FAIL (unchanged)** | durability-safe write strategy |
| 1M corpus / ANN M=64 build | Medium | **BLOCKED** | more RAM/time |
| Sufficiency is coverage-based (no semantics) | Medium | **Open** | embedding-based sufficiency |
| Trust depends on `source:` provenance metadata | Medium | **Open** | real provenance pipeline |
| Encoded injections beyond base64/zero-width | Medium | **NOT RUN** | broader decoding |
| Controls invalid (note-only prompt) | Low | **CONDITIONAL** | prompt fix (Phase 5) |
| Supersession misses paraphrase corrections | Medium | **Open** | semantic supersession |
| Large single-subject chain remains O(bucket²) | Low | **Open** | incremental supersession |
| Token-budget × trust composition not tested | Medium | **BLOCKED** | budget sweep in Phase 5 |

## 3. Design conflicts and how they were resolved

- **Speed vs correctness** in candidate discovery: a frequency cap was reverted because it
  broke correction chains. Correctness retained; residual worst case documented.
- **Ambiguity fix vs over-abstention**: first sufficiency attempt regressed 2 cases;
  corrected with stemming rather than abandoned. Neither "always abstain" nor
  "always answer" was adopted.
- **Security vs usefulness**: injection defense preserves mixed-memory facts (usefulness)
  while removing instructions (safety).

## 4. Recommended next engineering work (ordered)

1. Run the Phase 5 300-task campaign (the decision gate).
2. Add embedding-based sufficiency and paraphrase-aware supersession.
3. Investigate a durability-safe write-concurrency strategy for the 16-client tail.
4. Compose the trust layer with the token-budget selector and sweep budgets.
5. Broaden injection decoding (custom alphabets, homoglyph normalisation).
6. Real provenance pipeline so trust does not depend on a benchmark tag.

## 5. What must NOT happen

- Do not promote any mechanism to default before Phase 5 shows a significant,
  regression-free, safety-passing improvement.
- Do not trade durability or safety for task success.
- Do not tune on the Phase 5 held-out tasks.
