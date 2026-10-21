# Beacon Memory Engine v3 — architecture and status

Branch `feat/beacon-memory-engine-v3`, based on the verified v2 commit
`ee1cc516` (baseline `99c70908`). Upstream `main` was fetched and is still at
`99c70908`, so there are no incompatible changes to reconcile.

v3 preserves v2 in full: the adaptive FTS5 lexical engine, planner, bounded
generation-safe cache, transactional index maintenance, and recovery are
unchanged and still measured by `SEARCH_BENCHMARKS.md`. v3 adds three layers on
top, each feature-flagged and independently benchmarkable.

## Feature flags / entry points

| component | entry point | default |
|---|---|---|
| lexical adaptive search (v2) | `Store.ListMemories` | on, unchanged |
| semantic layer | `learning.OpenSemantic(path, embedder)` | off (`Open` has no embedder) |
| hybrid fusion | `Store.SearchHybrid(query, opts)` | returns lexical ranking when no embedder |
| evidence trust | `learning.AnnotateTrust(memories, opts)` | pure function, no store change |
| embedding backfill | `Store.RebuildEmbeddings()` | only when an embedder is set |

Public interfaces are additive. `Open`, `Query`, `Store.ListMemories` and the
existing MCP/CLI paths are unchanged, so the default experience is byte-for-byte
the v2 behaviour.

## Component 2 — semantic retrieval (PARTIAL)

- `Embedder` interface (`Model`, `Dim`, `Embed`) is the seam for a model backend.
- Default backend `HashedEmbedder` is dependency-free signed feature hashing over
  word tokens and character trigrams. It captures morphology and partial lexical
  overlap; it is **not** a neural model and does not capture true synonymy.
- `memory_embeddings(memory_id, project_id, model, content_hash, dim, vector)`
  stores one L2-normalized vector per memory, keyed by content hash and model so
  a content or model change refreshes only that memory (incremental).
- `SearchHybrid` retrieves lexical and semantic candidates independently and
  fuses them with reciprocal rank fusion (RRF), which avoids adding uncalibrated
  BM25 and cosine scores.
- Exact vector search only; no ANN (see SCALING_BENCHMARKS.md).
- A neural model backend is **not implemented**; the interface and storage are
  ready for one. See SEMANTIC_BENCHMARKS.md for measured quality.

## Component 3 — evidence trust (PARTIAL)

- `AnnotateTrust` labels each memory `verified`, `unverified`, `stale` or
  `contradicted`, from deterministic rules only: usable provenance, freshness
  window, and same-project/same-kind opposite-polarity conflicts.
- Approval and supersession are kept separate from evidence status; the layer
  never approves, rejects, or widens visibility.
- Model-based adjudication is **not implemented**. Statuses are labels, not
  calibrated probabilities.
- See TRUST_BENCHMARKS.md for measured precision/recall and leakage.

## Component 4 — reliability (PARTIAL, inherited from v2)

v2 already provides transactional index+generation writes, cross-process
generation-validated caching, fallback, rebuild, `_txlock=immediate` +
`_busy_timeout`, and write durability under derived-index loss, all covered by
tests run under `-race`. v3 adds semantic/trust state but does not regress these.
See RELIABILITY_BENCHMARKS.md for what is covered and what is not.

## Component 5 — scalable hybrid retrieval (PARTIAL)

The planner still selects exact scan / FTS / cache. The semantic source adds
exact vector search with a bounded top-k; no ANN is implemented because no ANN
was shown to beat exact search at the sizes measured here without recall loss.
See SCALING_BENCHMARKS.md.

## Phase 5 — agent task evaluation (NOT IMPLEMENTED)

No agent runtime, model endpoint, or tool harness is available in this
environment, and the default path must not depend on a hosted service. End-to-end
agent-task evaluation was therefore **not run**. AGENT_TASK_EVALS.md records the
design and states this explicitly. No agent-task numbers are claimed.

## Consolidated results (measured on this machine)

Machine: WSL2 Ubuntu, 8 vCPU, i5-1155G7 @ 2.50GHz, 7.8 GB RAM, Go 1.27.0,
modernc.org/sqlite v1.59.0.

| dimension | baseline | v2 | v3 |
|---|---|---|---|
| lexical selective cold 100K (p50) | 8.6 ms | 3.97 ms | 3.97 ms (unchanged) |
| lexical old-only cold 100K (p50) | 328 ms | 0.79 ms | 0.79 ms (unchanged) |
| warm cached 100K (p50) | 8.6 ms | 0.24 ms | 0.24 ms (unchanged) |
| semantic recall@5 (labeled set, n=17) | n/a | 0.41 (lexical) | 0.71 (semantic), 0.71 (hybrid) |
| semantic recall@1 | n/a | 0.12 (lexical) | 0.41 (semantic), 0.29 (hybrid) |
| contradiction precision / recall | n/a | n/a | 1.00 / 1.00 (n=8 labeled pairs) |
| cross-scope leakage | 0 | 0 | 0 |

Every value above is measured, not extrapolated. Reproduction commands are in
each benchmark document.
