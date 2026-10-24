# Upstream PR plan

## Scope of this branch

`feat/beacon-memory-engine-v3`, based on `ee1cc516` (v2). It adds an optional
semantic layer, a deterministic evidence-trust layer, and their tests, without
changing the default lexical path or any public memory semantics.

## Suggested split into reviewable PRs

1. **Semantic layer** — `search_embed.go`, `search_semantic.go`, `semantic_test.go`,
   `SEMANTIC_BENCHMARKS.md`. Adds `Embedder`, `OpenSemantic`, `SearchHybrid`,
   `RebuildEmbeddings`, and the `memory_embeddings` table (created idempotently;
   no migration of existing tables). Default `Open` is unaffected.
2. **Evidence trust** — `search_trust.go`, `trust_test.go`, `TRUST_BENCHMARKS.md`.
   Pure functions; no store change.
3. **Docs** — `MEMORY_ENGINE_ARCHITECTURE.md`, `SCALING_BENCHMARKS.md`,
   `RELIABILITY_BENCHMARKS.md`, `AGENT_TASK_EVALS.md`.

PR 1 is the only one that touches storage; PRs 2–3 are additive.

## Migration and compatibility

- `ensureSemanticSchema` uses `CREATE TABLE IF NOT EXISTS`; opening an existing
  v2 database adds the empty table and nothing else. Existing rows, the
  `memories` schema and `PRAGMA user_version` are untouched.
- `RebuildEmbeddings` backfills a store on demand; no automatic backfill runs on
  the default path.
- `Embedder.Model()` is stored per vector, so a model change refreshes only that
  model's vectors.

## Reviewer checklist

- Default path unchanged: `Open` + `ListMemories` behaviour identical to v2.
- No mandatory hosted service, no heavyweight dependency, no model download.
- Project isolation and approval workflow untouched.
- `go test ./internal/learning/...` and `go test -race ./internal/learning/...`
  pass; failing-test set equals baseline.
- Benchmarks reproducible per the commands in each document.

## Explicitly out of scope (candidate follow-ups)

- Neural embedding backend behind `Embedder`.
- ANN index above the measured break-even (~a few thousand memories).
- Weighted / reranked fusion beyond RRF.
- Model-based contradiction adjudication.
- Context token budgeting.
- End-to-end agent-task evaluation.

## Risks to call out in the PR

- `HashedEmbedder` is lexical, not neural; paraphrase recall is limited (measured).
- Exact semantic search is linear and not viable at 100K (measured).
- Contradiction detection is a small English heuristic; precision on real data
  is unmeasured.
