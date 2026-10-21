# Vector scaling report

Status: exact search measured to 100K; ANN candidate measured and **rejected**;
1M semantic **not viable / not measured**.

Machine: WSL2 Ubuntu, 8 vCPU, i5-1155G7, 7.8 GB RAM, Go 1.27.0.

## Exact semantic search (measured, `BenchmarkSemanticExact`)

Top-5 cosine over all project embeddings, including SQLite load, BLOB decode,
JSON decode, scoring.

| n | ms/op | B/op |
|---|---|---|
| 1,000 | 5.49 | 4,931,178 |
| 10,000 | 65.8 | 49,138,294 |
| 100,000 | 513.3 | 490,847,240 |

Linear in corpus size and dominated by decoding every candidate. Viable for
small stores (single-digit ms at 1K), **not viable at 100K**. 1M was not run
because it is strictly worse and would not fit the memory budget with 512-dim
float32 vectors in RAM.

## ANN candidate: coder/hnsw v0.6.1 (measured)

Latency (`BenchmarkANNvsExact`, in-memory):

| n | ANN p50 | exact p50 | ANN speedup |
|---|---|---|---|
| 1,000 | 34 µs | 222 µs | 6.5× |
| 10,000 | 47 µs | 2.64 ms | 56× |
| 100,000 | 76 µs | 30.2 ms | 397× |

Recall against exact ground truth on structured clustered vectors (well-separated
clusters, so exact NN is well-defined):

| EfSearch | recall@10 |
|---|---|
| 20 | 0.172 |
| 40 | 0.172 |
| 100 | 0.172 |
| 200 | 0.172 |
| 400 | 0.172 |

Recall is ~0.17 and **does not change with `EfSearch`**, which indicates the
graph search is not navigating (a shallow, near-constant result set), not merely
under-tuned. Latency is excellent but accuracy is unacceptable, so the candidate
is **REJECTED**. On the hashed-embedder vectors recall is 0.000 (flat space).

## Exact vs ANN decision

- Exact search is the correctness reference and the only mode with proven recall.
- No ANN candidate passed the recall gate. **ANN is not selected.**
- Break-even: exact search is acceptable up to roughly a few thousand memories;
  above that, retrieval is slow and no validated replacement exists.

## Index lifecycle

- ANN graphs are rebuilt in-process when the durable generation changes; no
  automatic persistence/load is wired (Export/Import exist in coder/hnsw).
- The derived embedding table is rebuildable via `RebuildEmbeddings`.

## Not measured

- 1M semantic retrieval (exact or ANN).
- ANN disk footprint and cold-start (in-memory only here).
- ANN update/delete cost.
