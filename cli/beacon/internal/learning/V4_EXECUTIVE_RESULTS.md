# V4 Executive Results

Branch `research/beacon-memory-engine-v4` (from `3065c210`). Upstream `main` is
still `99c70908`; no incompatible changes. Baseline `99c70908` untouched.

This phase integrated and measured candidate components. Every number below is
measured on one machine (WSL2 Ubuntu, 8 vCPU, i5-1155G7 @ 2.50GHz, 7.8 GB RAM,
Go 1.27.0, modernc.org/sqlite v1.59.0) unless marked otherwise.

## Requested comparison

| Dimension | Original Beacon | Best experimental V4 | Improvement |
|---|---|---|---|
| Retrieval accuracy | lexical recall@5 0.412 (17-case labeled set) | semantic (hashed) recall@5 0.706 | +71% relative |
| p95 retrieval latency (100K, old-only) | 361,635 µs | 1,206 µs (v2 indexed) | ~300× |
| Search at 1M | old-only p50 3,387,032 µs | lexical p50 1,597 µs; semantic exact 513 ms @100K (1M not viable); ANN rejected | ~2000× lexical; semantic at 1M **not proven** |
| Evidence trust | none (human approval only) | deterministic labels + contradiction rules; P/R 1.00 on 8 labeled pairs | new capability |
| Cache correctness | n/a | verified under `-race` + fault tests; zero cross-scope leakage | verified |
| Context tokens (17 queries) | fixed top-5: 2,140 tokens | budgeted: 1,242 tokens | 1.72× fewer |
| Agent task success | **NOT MEASURED** | **NOT MEASURED** | not measured |
| Disk/RAM overhead (100K) | 64.9 MB | 136.5 MB (trigram index) | 2.1× disk tradeoff |

## Decisions (evidence-based)

| Candidate | Accuracy | Latency | Memory/Storage | Reliability | Decision |
|---|---|---|---|---|---|
| v2 FTS5 lexical + planner + cache | high (exact) | 0.24–3.97 ms @100K | index 2.1× disk | verified | **SELECT** (kept) |
| Hashed-subword semantic | recall@5 0.71 | exact search 5.5–513 ms | vectors ~2 KB/mem | derived, rebuildable | **SELECT** (small stores only) |
| coder/hnsw ANN v0.6.1 | recall@10 0.17 (insensitive to EfSearch) | 76 µs @100K | in-memory | unproven | **REJECT** |
| Neural embeddings (fastembed/ONNX) | — | — | — | not integrable here | **NOT IMPLEMENTED** |
| sqlite-vec / hnswlib | — | — | cgo/extension | — | **NOT INTEGRATED** |
| Deterministic trust validators | P/R 1.00 (n=8) | microseconds | none | pure functions | **SELECT** |
| Token budgeting | gold retained 11/17 vs 12/17 | microseconds | none | pure function | **SELECT** |
| LLM contradiction adjudication | — | — | — | no model | **NOT IMPLEMENTED** |

## Major limitations

- No neural semantic model: true-synonym paraphrases remain unsolved (recall 0 on
  the `synonym` category). The semantic win is on partial-overlap paraphrases.
- Exact semantic search is linear (513 ms @100K); the one ANN candidate tried was
  rejected on recall. Semantic retrieval above ~10K is **unproven**.
- Agent-task success was **not measured**: no agent runtime/model is available
  and the default path must not require a hosted service.
- Trust and contradiction rules are small heuristics; precision on real data is
  unmeasured.

## Recommended architecture (pending review)

Keep v2 lexical as the always-on default. Add the semantic layer as opt-in for
stores up to a measured break-even (~10K memories) using exact search. Keep
deterministic trust annotation and token budgeting. Do **not** ship ANN until a
candidate passes a recall gate. Do not ship a neural model until one can be
bundled legally and run offline.

Per the brief, the final production build is **not** implemented; this is the
experimental result set for review.
