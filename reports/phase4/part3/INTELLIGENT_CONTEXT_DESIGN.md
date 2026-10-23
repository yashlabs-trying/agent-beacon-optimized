# Phase 4C — Intelligent Context Design

The layer: `cli/beacon/internal/learning/context_select.go`, entry point
`SelectTrustContext(memories, query, opts) TrustContext`. It runs **after** retrieval and
**before** agent context assembly. It is **opt-in**; no default behaviour changed.

## Pipeline

1. Retrieve candidate memories (semantic/lexical — unchanged).
2. **Injection defense** (`EnableInjectionDefense`): flag memories whose bodies contain
   embedded instructions; strip instruction sentences and keep useful facts; drop a
   memory only if nothing factual remains.
3. **Trust tiering** (`EnableTrust`): flag untrusted provenance (tag `source:untrusted`
   / `source:external` / `source:injection`, or body markers such as "untrusted",
   "external document", "forged"); drop untrusted memories **unless** they were sanitized
   to retain a useful fact.
4. **Supersession** (`EnableSupersession`): honour the `SupersededBy` field and detect
   explicit corrections — a newer memory (by timestamp) that carries a correction marker
   ("now", "changed to", "no longer", "replaced", "migrated to", …) and shares a
   subject with an older memory (≥2 significant terms and ≥ half of the smaller memory's
   terms). A correction from an **untrusted** source is never applied.
5. **Conflict** (`EnableConflict`): reuse `detectContradictions`; drop the losing side
   when the other side is superseded/untrusted; otherwise **preserve both** and set
   `Signals.Unresolved = true`.
6. **Budget**: cap the result count, preferring verified memories.

## Design rules honoured

| Rule | Implementation |
|---|---|
| Do not blindly favour the newest | Supersession requires an explicit correction marker + shared subject; untrusted newer corrections are rejected |
| Do not blindly favour confidence | No confidence is used; only provenance, scope, and explicit relationships |
| Prefer verifiable relationships | same project, compatible version (`distinctExplicitVersions`), supported correction, provenance, temporal applicability |
| Preserve unresolved uncertainty | Unresolved conflicts keep both sides and raise a signal |
| No instructional authority | Bodies are treated as data; injected instructions are stripped; the agent prompt is unchanged across arms |

## Output

```go
type TrustContext struct {
    Memories []asymptoteobserve.LearningMemoryV1
    Signals  TrustContextSignals // superseded, conflicts, untrusted, injection, sanitized, unresolved
}
```

## Known limitations (reported honestly)

- Supersession/conflict are keyword- and token-based, not semantic: paraphrases with
  little lexical overlap are missed (`cx_temporal_obsolete` in the first dev build).
- The supersession scan is O(n²) pairwise (see `MEMORY_RELIABILITY.md`).
- The layer does not resolve cross-kind or negation conflicts where both sides are
  credible — it correctly declines, but the agent may still guess.
- On unresolvable ambiguity the layer preserves the (insufficient) hint rather than
  withholding it, which can push the agent to over-answer — an open defect.
