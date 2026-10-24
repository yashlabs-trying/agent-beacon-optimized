# Phase 4A — Benchmark B: Retrieval Baseline Comparison

Independent retrieval comparisons on **identical** corpus contents, identifiers,
queries, relevance labels, candidate limits, and hardware.

- Fixture: `reports/phase3/heldout/retrieval_context_v1.json` (12 topics, 48 queries,
  48 memories; gold = `{primary, duplicate}`), SHA-256 `FADF313E…114A`.
- Beacon methods: Phase 3A harness (`phase3a_retrieval_test.go`,
  `phase3a_semantic_test.go`); neural via the Mac M5 sidecar.
- Independent baselines: `reports/phase4/part1/harness/retrieval_baselines.py`
  (BM25, SQLite FTS5, recency, exact-BGE oracle).
- Raw evidence: `RAW_EVIDENCE/retrieval_neural_mac.txt`,
  `RAW_EVIDENCE/retrieval_baselines.{txt,json}`.

## Locked fixture (limit=10)

| Method | R@1 | R@5 | R@10 | P@5 | MRR | nDCG@5 | p50 | p95 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Original lexical (`99c70908`) | 0.167 | 0.167 | — | — | 0.167 | 0.134 | 0.54 ms | 0.98 ms |
| V6 lexical | 0.167 | 0.167 | — | — | 0.167 | 0.134 | 0.57 ms | 1.24 ms |
| V6 hashed-semantic | 0.688 | 0.875 | — | — | 0.764 | 0.716 | 1.08 ms | 4.06 ms |
| V6 hybrid RRF | 0.688 | 0.875 | — | — | 0.764 | 0.716 | 0.99 ms | 2.65 ms |
| V6 hybrid ANN | 0.688 | 0.875 | — | — | 0.764 | 0.716 | 0.82 ms | 1.73 ms |
| **V6 real neural BGE-small** | **0.646** | **0.979** | **1.000** | — | **0.785** | **0.789** | 43.6 ms¹ | 133 ms¹ |
| BM25 (independent) | 0.688 | 0.875 | 0.979 | 0.317 | 0.773 | 0.735 | 0.13 ms | 0.61 ms |
| SQLite FTS5 (independent) | **0.708** | **0.917** | 0.958 | 0.325 | 0.788 | 0.753 | 0.28 ms | 0.39 ms |
| BGE exact cosine (oracle) | 0.750 | 1.000 | 1.000 | 0.379 | 0.848 | 0.863 | 57.6 ms¹ | 113 ms¹ |
| Recency-only | 0.000 | 0.083 | 0.167 | 0.033 | 0.040 | 0.048 | 0.001 ms | 0.001 ms |

¹ Includes LAN round-trips to the Mac sidecar (WSL2 client → M5 MPS). Not an on-host latency.

### Findings

- **The V6 lexical path equals original Beacon** (both 0.167 R@5) — they share the same
  AND-substring predicate; the difference is the indexed execution path, not ranking.
- **The hashed-semantic arm is not better than simple independent baselines.** SQLite
  FTS5 reaches R@5 **0.917 > 0.875** (and higher R@1, MRR, nDCG) at *lower* latency.
  BM25 ties hashed at R@5 0.875. This is the strongest negative result in Phase 4A.
- **Only the real neural arm clearly leads**: R@5 0.979 (V6 ANN over BGE) vs FTS5 0.917
  and lexical 0.167. The exact-cosine oracle (0.100 R@5) bounds the ANN gap.
- Recency-only is near-useless (0.083), confirming the fixture rewards content matching.

## Synthetic scale sweep (clean labels, 200 queries)

Corpus: `doc{i}` unique-token documents; query = the unique token; gold = that document.

| Scale | Method | R@1 | R@5 | R@10 | p50 | p95 | p99 | build |
|---:|---|---:|---:|---:|---:|---:|---:|---:|
| 1K | BM25 | 1.000 | 1.000 | 1.000 | 0.54 ms | 1.20 ms | 3.13 ms | 21 ms |
| 1K | FTS5 | 1.000 | 1.000 | 1.000 | 0.018 ms | 0.032 ms | 0.070 ms | 8 ms |
| 1K | recency | 0.000 | 0.005 | 0.010 | 0.000 | 0.000 | 0.001 | 0 ms |
| 10K | BM25 | 1.000 | 1.000 | 1.000 | 8.74 ms | 17.95 ms | 23.19 ms | 110 ms |
| 10K | FTS5 | 1.000 | 1.000 | 1.000 | 0.021 ms | 0.050 ms | 0.091 ms | 49 ms |
| 10K | recency | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0.001 | 0.3 ms |
| 100K | BM25 | 1.000 | 1.000 | 1.000 | 56.8 ms | 99.5 ms | 152.8 ms | 1,031 ms |
| 100K | FTS5 | 1.000 | 1.000 | 1.000 | 0.033 ms | 0.051 ms | 0.073 ms | 498 ms |
| 100K | recency | 0.000 | 0.000 | 0.000 | 0.000 | 0.001 | 0.002 | 2.4 ms |

Interpretation: with exact-match labels both BM25 and FTS5 are perfect; **FTS5 is
1–3 orders of magnitude faster than pure-Python BM25** at every scale and is the fair
"simple independent baseline" for the locked set. No 1M ANN build was run.

## Constraints honoured

- No 1M ANN build; bounded to 100K.
- Same labels/budgets across methods; no cross-definition score mixing.
- Locked queries used for regression only (the fixture is Phase 3-locked); the 17-case
  and 96-case sets are dev-only and reported separately in `EVIDENCE_AUDIT.md`.

## Conclusion

**Benchmark B: PASS (executed).** V6's semantic layer beats original lexical, but the
**hashed arm is dominated by a trivial FTS5 baseline**; only the real neural arm provides
a clear, independent retrieval win.
