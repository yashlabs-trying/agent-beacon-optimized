# Semantic retrieval benchmarks (Component 2)

Status: **PARTIAL**. A working, dependency-free semantic layer with measured
quality; a neural embedding backend is not implemented.

## Setup

- Machine: WSL2 Ubuntu, 8 vCPU, i5-1155G7, 7.8 GB RAM, Go 1.27.0.
- Embedder: `HashedEmbedder` (`hashed-subword-v1`), 512 dims, signed feature
  hashing over words + character trigrams, L2-normalized. No training, no
  downloads, no network.
- Store: `OpenSemantic` (embedder enabled), embeddings written incrementally in
  `PutMemory`.
- Test: `TestSemanticRetrievalQuality`, `TestHashedEmbedderBasics`,
  `TestSemanticIncrementalUpdate`, `TestHybridNoModelFallback`.

## Labeled held-out set

17 hand-written cases in four categories, with three misleading distractors.
The set was authored for this test; the embedder has no training step, so there
is no train/tune/test contamination.

- `overlap` (8): paraphrase keeps shared technical terms.
- `exact` (3): query is a near-verbatim phrase from the gold memory.
- `synonym` (3): true paraphrase with no shared substrings.
- `misleading` (3): the query shares terms with a wrong memory that outranks the
  gold.

## Results (measured)

| method | recall@1 | recall@5 | MRR | nDCG@5 |
|---|---|---|---|---|
| lexical (v2) | 0.118 | 0.412 | 0.235 | 0.280 |
| semantic-only | 0.412 | 0.706 | 0.549 | 0.590 |
| hybrid (RRF) | 0.294 | 0.706 | 0.490 | 0.546 |

Per category (recall@1 / recall@5):

| method | overlap | exact | synonym | misleading |
|---|---|---|---|---|
| lexical | 0.25 / 0.50 | 0.00 / 1.00 | 0.00 / 0.00 | 0.00 / 0.00 |
| semantic | 0.75 / 1.00 | 0.33 / 1.00 | 0.00 / 0.33 | 0.00 / 0.00 |
| hybrid | 0.50 / 1.00 | 0.33 / 1.00 | 0.00 / 0.33 | 0.00 / 0.00 |

## Interpretation (honest)

- The semantic layer improves recall@5 from 0.41 to 0.71 and recall@1 from 0.12
  to 0.41 over the lexical baseline **on this set**. This is the strongest
  evidence the interface, storage and fusion are correct and useful.
- Semantic-only beats hybrid on recall@1 here: RRF dilutes a strong semantic top
  hit with weaker lexical ranks. Hybrid matches semantic on recall@5. On a set
  where lexical is the stronger source, the ordering would reverse.
- `synonym` is near-zero for every method. This is the expected limitation of a
  lexical embedder: it cannot connect "login credential lifetime" to
  "authentication token expiry". A neural model is required and is **not
  implemented**.
- `misleading` is zero for every method: a near-verbatim distractor outranks the
  true paraphrase. Lexical-overlap traps defeat all three methods; a reranker or
  trust layer would be needed.

## Latency and overhead

- Embedding generation: local, ~microseconds per memory; measured as part of
  write cost (see SCALING_BENCHMARKS.md).
- Semantic exact search latency scales with corpus size; see
  SCALING_BENCHMARKS.md. Retrieval latency is reported separately from embedding
  generation there.

## Reproduction

```bash
cd cli/beacon
go test -run 'TestSemantic|TestHashed|TestHybrid' -v ./internal/learning/
go test -run '^$' -bench BenchmarkSemanticExact -benchmem ./internal/learning/
```

## Not implemented

- Neural embedding backend (e.g. a small ONNX sentence model).
- Weighted or reranked fusion beyond RRF.
- A larger, independently authored relevance set; 17 cases is small and the
  confidence intervals are wide.
