# Retrieval pipeline

A focused walkthrough of the question-aware retrieval path that the optimization work
touches. For the visual overview see [architecture](architecture.md).

## Entry points

- **CLI:** `beacon memory list --question-aware --question "<q>"` →
  `runMemoryList` → `SearchQuestion`.
- **MCP:** the `search_memory` / `get_memory_context` tools served by
  `beacon mcp serve`.

The **default** path (`memory list` without `--question-aware`) still uses
`ListMemories`. The optimization does not change that default.

## Stages

### 1. Scope

`scopeQuery` resolves the project (ID or path). A missing scoped project, question, or
query is an explicit error — the pipeline never silently widens the scope.

### 2. Pool + schema

`db()` lazily opens SQLite and runs `ensureSchemaOnce` (authoritative + derived tables,
WAL). A per-process cache avoids repeated schema work; a new CLI process re-checks.
`busy_timeout=30000`, `BEGIN IMMEDIATE` for writers.

### 3. Lexical index (FTS5 trigram)

`ensureIndexReady` may build the derived `memories_fts` trigram index on first use.
`queryIsSelective` checks term selectivity; unselective/short terms fall back to a
bounded authoritative scan. `fetchIndexedCandidates` joins FTS to `memories` and
JSON-decodes up to the candidate cap.

> **Optimization (P1 / `92e910bf`):** the trigram selectivity check is batched for long
> multi-term queries instead of issuing one statement per term.

### 4. Cache

A bounded (32 MiB) LRU keyed on path/project/kind/raw-query/limit/page/ranking-version.
A durable **generation** counter changes on every write; entries are checked against it
before serving, so the cache cannot return stale results after a write or supersede.

### 5. Trust pass and relevance

`SelectTrustContext` applies injection handling, source heuristics, and supersession
**before** relevance. `analyzeQuestion` classifies direct evidence vs a bridge/link for
a second hop. This is the layer the allocation work optimized.

> **Optimization (Phase 2 / `90b21107`):** `significantTerms` is computed once per
> memory and shared across supersession and contradiction detection; `sharedCount`
> avoids allocating an intersection slice when only the count matters; supersession
> tokenization is skipped entirely when no memory can supersede. **2,007 → 875
> allocs/op** with identical pair/supersession/contradiction decisions.

### 6. Conditional semantic expansion

Only when trusted **lexical** evidence does not directly support the question **and** an
embedder is explicitly configured. `--semantic` uses the offline **hashed** embedder.
ANN and neural retrieval stay opt-in and off by default.

### 7. Final trust, budget, sufficiency

A second `SelectTrustContext` pass runs on the selected evidence with conflict enabled.
`selectWholeQuestionContext` counts title/body tokens and keeps **whole memories**,
never a misleading fragment, and emits a sufficiency signal.

### 8. Return

`Close` the store, JSON-encode (or text), return to the caller/agent. The route and
embedding model are emitted in JSON when question-aware search is used.

## Write path (what the concurrency fix changes)

`PutMemory` marshals JSON, acquires the process-level **`writeMu`**, opens
`BEGIN IMMEDIATE`, upserts the authoritative row, updates FTS if healthy, bumps the
durable generation (invalidating cache and derived vectors), upserts/invalidates the
embedding, and commits.

> **Optimization (Phase 1 / `d32e9d3b`):** `writeMu` makes only one goroutine hold a
> SQLite write transaction at a time. Others park on a cheap Go mutex instead of
> spinning in SQLite's busy handler. Durability is unchanged; the concurrency tail
> collapses (16-client p99 **634 → 48–54 ms**).

## Invariants (preserved by the optimizations)

- Released/selection semantics unchanged; trust, injection, supersession, conflict, and
  sufficiency all still run.
- Project scoping is applied to both SQL and the semantic query.
- Cache generation invalidation unchanged (`TestSearchCacheInvalidatedByWrite`,
  `…BySupersede`, `TestPhase2CacheInvalidationUnderWrites`).
- The default (non-question-aware) path is unchanged.

## Source map

| Stage | File |
|---|---|
| Orchestration | `search_question.go` |
| FTS index readiness | `search_question_index.go` |
| Cache | `search_cache.go` |
| Trust / supersession / contradiction | `context_select.go` |
| Store + write serialization + generation | `store.go` |
| Semantic | `search_semantic.go`, `internal/hnsw` |
