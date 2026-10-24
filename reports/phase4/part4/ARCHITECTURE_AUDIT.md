# Phase 4D — Architecture Audit

## 1. Data path (as built)

```
ingestion (candidates → approval)
  → storage (SQLite: memories, FTS index, embeddings, generations)
  → indexing (lexical FTS; hashed/neural embeddings; optional HNSW ANN)
  → retrieval (ListMemories lexical | SearchHybrid hashed/hybrid/neural)
  → [candidate ranking — inside SearchHybrid RRF]
  → trust classification (AnnotateTrust)          ─┐
  → supersession / contradiction (search_trust.go) ─┤  Phase 4C/4D: SelectTrustContext
  → evidence sufficiency (Phase 4D)                ─┤  (single post-retrieval owner)
  → context selection + budget                     ─┘
  → agent prompt construction (harness / CLI / MCP)
  → tool usage → task outcome
```

## 2. Stage ownership and duplication

| Stage | Owner | Notes |
|---|---|---|
| Retrieval | `Store.ListMemories`, `Store.SearchHybrid` | lexical vs semantic |
| Ranking | RRF inside `SearchHybrid` | weightable (`LexicalWeight`/`SemanticWeight`) |
| Trust | `AnnotateTrust` (search_trust.go) | status labels |
| Contradiction detection | `detectContradictions` (search_trust.go) | trusted Phase 3 code |
| Supersession / conflict / injection / sufficiency | `SelectTrustContext` (context_select.go) | **single owner after Phase 4D** |
| Token budget | `SelectContext` (search_context.go) | separate; not yet composed with trust |

**Duplication found:** before Phase 4D, contradiction logic existed in `detectContradictions`
and was re-implemented pairwise inside `SelectTrustContext`. Phase 4D routes selection
through one indexed detector (`detectContradictionsIndexed`, equality-tested against the
trusted one).

## 3. Critical findings

1. **The intelligent/trust layer was unreachable in production.** `SelectTrustContext`,
   `AnnotateTrust`, and `SelectContext` had **zero non-test callers**; the CLI
   (`cmd/memory.go`), dashboard (`internal/endpoint/dashboard/memory.go`), and MCP server
   (`internal/mcpserver/server.go`) call only `ListMemories`/`SearchHybrid`. The new
   intelligence could not be exercised through any supported interface.
2. **The CLI exposes only lexical retrieval.** No semantic/ANN flags exist; semantic
   retrieval is API-only. So even with the layer wired, natural-language queries return
   nothing (lexical AND-substring).
3. **O(n²) pairwise scans** in `detectContradictions` and supersession: correctness-safe
   but quadratic in the candidate-set size.
4. **Provenance loss risk:** the token-budget selector truncates bodies; it preserves
   evidence entries but is not composed with the trust layer, so a truncated correction
   could hide the fact that a statement is disputed (Invariant G risk).
5. **Untrusted content instructional authority:** the agent prompt is built by the
   harness; retrieved text is included as data, but there was no structural boundary
   beyond a prompt sentence, and no obfuscation handling.
6. **Locks/shared state:** `Store` uses a per-Store `*sql.DB` pool (Phase 3C); the
   selector is a pure function with no shared state (no new locks).
7. **Config divergence:** trust/neural/ANN remain opt-in; no default changed.

## 4. Changes made in Phase 4D (all opt-in)

- Made the layer reachable: `memory list --semantic` (offline hashed retrieval) and
  `--intelligent` / `--sufficiency` (Phase 4D selection). Defaults unchanged.
- Replaced the O(n²) pair scan with an inverted-index candidate discovery (identical
  results; 31.5× faster at n=1000).
- Structured supersession (`SupersededBy` + chains + cycle break).
- Evidence-sufficiency signal with light stemming.
- Injection hardening (encoded/zero-width detection).
- 12 cross-component contract tests.

## 5. Expected improvements vs possible regressions

| Change | Expected | Regression risk | Mitigation |
|---|---|---|---|
| Indexed candidate discovery | speed | missed pairs | equality test vs naive; no frequency cap |
| Sufficiency withholding | fewer guesses | over-abstention | light stemming + threshold tuned on dev; held-out unchanged |
| CLI flags | reachability | default change | flags default false; `cmd` tests pass |
| Encoded injection detection | safety | false positives on base64 data | only flags decoded instruction keywords |

## 6. Dependency graph (selection layer)

```
memories ─► injection scan ─► trust tier ─► supersession (indexed pairs)
         ─► conflict (indexed pairs) ─► sufficiency ─► budget ─► TrustContext
                                                  └─► Signals (superseded, conflicts,
                                                      untrusted, injection, unresolved,
                                                      sufficient/insufficient, note)
```
