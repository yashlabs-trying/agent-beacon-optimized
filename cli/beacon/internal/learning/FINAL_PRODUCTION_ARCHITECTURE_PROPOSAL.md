# Final production architecture proposal (for review, not built)

This is a recommendation based on the measurements in this branch. Per the
brief, the final production build is intentionally **not** implemented.

## Selected technologies

1. **Authoritative store**: existing SQLite `memories` table (unchanged).
2. **Lexical retrieval**: existing FTS5 trigram index + planner + generation-safe
   cache (v2). Always on; no model, no network.
3. **Semantic retrieval (optional, small stores)**: `HashedEmbedder` behind the
   `Embedder` interface with exact cosine search, enabled only via
   `OpenSemantic`, recommended below the measured break-even (~a few thousand
   memories).
4. **Fusion**: RRF, but the evidence says semantic-only beats RRF on recall@1;
   ship fusion only with weights tuned on a held-out set (not yet done).
5. **Trust**: deterministic evidence-status + contradiction rules; always on,
   pure functions.
6. **Context**: token-budgeted selection with body truncation; evidence preserved.
7. **ANN**: none selected.
8. **Neural embeddings**: none selected.

## Data flow

```
query -> lexical (FTS/planner/cache)  -> ranked lexical
      -> [optional] semantic exact    -> ranked semantic
      -> [optional] RRF fusion
      -> trust annotation (deterministic)
      -> token-budgeted context selection
      -> agent
```

## Storage design

- `memories` authoritative; `memories_fts`, `memory_index_meta`,
  `memory_trigram_df`, `memory_embeddings` derived and rebuildable.
- Migrations are `CREATE TABLE IF NOT EXISTS`; no existing table is altered.

## Fallback plan

- No embedder → lexical only (default).
- Embedding failure → derived table ignored; writes still succeed.
- Index corruption/deletion → scan fallback + rebuild.
- ANN not shipped, so no ANN fallback needed.

## Performance budgets (measured)

- Lexical warm query @100K: < 0.3 ms.
- Lexical cold selective @100K: < 5 ms.
- Semantic exact query: < 10 ms (≤ ~1–2K memories only).
- Write: < 1 ms.

## Operational risks

- Exact semantic search does not scale; enabling it on a large store is a
  footgun. Gate it by store size or refuse above the break-even.
- RRF can lower recall@1; must not be enabled without tuning evidence.
- Disk grows ~2.1× with the trigram index.

## What worked

FTS5 lexical + planner + cache; deterministic trust; token budgeting; the
`Embedder` seam and exact semantic search on small stores.

## What failed

coder/hnsw ANN (recall 0.17); neural embedding integration in this environment;
hybrid RRF on recall@1.

## Remaining blockers

- A validated ANN (pure-Go or vendored, license-cleared) with recall ≥ 0.9.
- A legally bundleable, offline neural embedding backend.
- A held-out tuning set large enough to weight fusion safely.
- End-to-end agent evaluation.
- Crash-during-write and soak testing.
