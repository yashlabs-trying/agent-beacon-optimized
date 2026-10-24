# Phase 4D — Memory Semantics and Invariants

## 1. Single decision contract

All post-retrieval decisions are made by one function, `SelectTrustContext`, which
returns a `TrustContext`:

```go
type TrustContext struct {
    Memories []asymptoteobserve.LearningMemoryV1 // curated, ordered evidence
    Signals  TrustContextSignals                 // every decision, observable
}
type TrustContextSignals struct {
    SupersededIDs, UntrustedIDs, InjectionIDs, SanitizedIDs []string
    ConflictPairs [][2]string
    Unresolved    bool
    Sufficient, Insufficient bool
    Missing       []string
    Uncertainty   []string
    SufficiencyNote string
}
```

No other component re-derives trust, supersession, conflict, or sufficiency. Retrieval
owns ranking; the layer owns curation. The token-budget selector remains a separate,
composable concern.

## 2. The ten invariants and how they are enforced/tested

| Invariant | Enforcement | Test |
|---|---|---|
| **A Scope isolation** | conflict/supersession require same `Project.ID` and compatible version (`distinctExplicitVersions`) | `TestContract_ScopeIsolation` |
| **B Supersession correctness** | only an explicit correction marker + shared subject from a trusted source supersedes; `SupersededBy` honoured; cycles broken | `TestContract_SupersessionKeepsHistory`, `TestContract_UntrustedCannotSupersede` |
| **C Conflict visibility** | unresolved conflicts keep both sides and set `Unresolved` + `Uncertainty` | `TestContract_BudgetKeepsConflictSignal` |
| **D Provenance preservation** | selection never strips `Evidence`; superseded records remain in the input | `TestContract_SupersessionKeepsHistory` |
| **E Trust separation** | retrieved text is data; injected instructions stripped/dropped; never promoted to instructions | `TestContract_InjectionNoAuthority` |
| **F Evidence sufficiency** | coverage-based sufficiency signal; high relevance ≠ sufficient | `TestContract_InsufficientEvidence` |
| **G Budget safety** | budget applies after supersession/conflict, so it cannot resurrect a superseded fact or drop a dispute signal | `TestContract_BudgetDoesNotResurrectSuperseded`, `TestContract_BudgetKeepsConflictSignal` |
| **H Lifecycle/durability** | layer is a pure function; storage unchanged | full suite + race (see `INTEGRATED_REGRESSION_RESULTS.md`) |
| **I Backward compatibility** | all flags default false; no default changed | `cmd` tests; `TestContract_NoOpWhenDisabled` |
| **J Fail-safe** | empty/malformed memory is not sufficient; unknown trust not promoted | `TestContract_MalformedNotTrusted` |

## 3. Decision order (documented, single source)

1. Injection scan → sanitize or drop.
2. Trust tiering → untrusted set.
3. Supersession (explicit + indexed lexical) → superseded set, cycle-safe.
4. Conflict (indexed) → resolved drops or unresolved signal.
5. Assemble survivors (drop injection/ superseded / resolved-conflict / untrusted).
6. Sufficiency assessment (coverage + stemming).
7. Budget (prefer verified).

Where a decision genuinely depends on context, the layer propagates an uncertainty
result (`Unresolved`, `Insufficient`) rather than inventing a rigid rule.

## 4. Note on "confidence vs truth"

The layer never uses a confidence score. It uses only: provenance tags, explicit
relationships, project/version scope, temporal applicability, and token coverage. This
upholds the brief's rule not to equate confidence with truth.
