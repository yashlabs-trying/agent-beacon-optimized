# Scaling benchmarks (Component 5)

Status: **PARTIAL**. Exact semantic search is measured at 1K–100K; ANN is
**not implemented** because no approach was shown to be justified at these
sizes without recall loss, and no ANN library was integrated.

## Machine

WSL2 Ubuntu, 8 vCPU, i5-1155G7 @ 2.50GHz, 7.8 GB RAM, Go 1.27.0,
modernc.org/sqlite v1.59.0.

## Lexical retrieval scaling

The v2 lexical engine (exact scan, FTS, cache) is measured at 1K/10K/100K/1M in
`SEARCH_BENCHMARKS.md`. v3 does not change it. Headline p50 at 100K: selective
cold 3.97 ms, old-only cold 0.79 ms, no-match cold 0.48 ms, warm 0.24 ms; at 1M:
old-only cold 1.60 ms, no-match cold 0.61 ms.

## Semantic exact search scaling (measured)

`BenchmarkSemanticExact` — top-5 exact cosine over all project embeddings,
including SQLite load, BLOB decode, JSON decode and scoring.

| n | ns/op | ms | B/op |
|---|---|---|---|
| 1,000 | 5,487,704 | 5.49 | 4,931,178 |
| 10,000 | 65,834,905 | 65.8 | 49,138,294 |
| 100,000 | 513,328,925 | 513.3 | 490,847,240 |

Retrieval latency is reported separately from embedding generation and index
construction:

- Embedding generation is local and cheap per memory; it is paid once per
  content/model change (incremental), measured as part of write cost in
  `SEARCH_BENCHMARKS.md`.
- Backfill (`RebuildEmbeddings`) is batched; not separately timed here.

## Interpretation

Exact semantic search is linear in corpus size and dominated by loading and
decoding every candidate vector. It is acceptable for small stores (single-digit
milliseconds at 1K) but **not viable at 100K** (513 ms, 490 MB/op), and 1M was
not run because it is plainly worse. This is the measured break-even evidence:
above roughly a few thousand memories, exact vector search needs an index.

## ANN evaluation

Two families were considered: random-projection LSH and IVF (k-means posting
lists) over the stored vectors. Neither was implemented, because:

- No local, maintainable ANN component compatible with the SQLite-only
  architecture was available without a new heavy dependency.
- The measured recall loss of a hand-rolled ANN could not be validated against
  exact ground truth at 1M within this pass.

Per the brief, ANN was **not** added for completeness. The plan is in
`UPSTREAM_PR_PLAN.md`.

## Context token budgeting

**Not implemented.** No token-budget-aware context assembly was added.

## Reproduction

```bash
cd cli/beacon
go test -run '^$' -bench BenchmarkSemanticExact -benchmem ./internal/learning/
BEACON_VERIFY=1 BEACON_VERIFY_SIZES=1000,10000,100000,1000000 \
  go test -run TestVerifyHarness -v ./internal/learning/   # lexical scaling
```
