# Phase 3A — Real Neural Retrieval (Workstream A)

**Status: PARTIAL. Neural arm BLOCKED; lexical, V6-lexical, hashed-semantic, and hybrid
arms measured on the locked 48-query set.**

## 1. Why neural is blocked

| Requirement | Result |
|---|---|
| Cached `BAAI/bge-small-en-v1.5` on the test host | **Absent** (HF cache holds only Qwen2.5-0.5B, SmolLM2-135M, opt-125m, gpt-oss-20b) |
| `torch` + `sentence-transformers` | **Not installed** on WSL2 (`import` fails) |
| Apple M5 sidecar host | **Unreachable** — `ping 192.0.2.4` → Destination Host Unreachable; `ssh` → No route to host |

Per the brief, an unavailable sidecar/model is marked **BLOCKED** rather than
substituted. No new model was downloaded (out of scope / explicitly discouraged). The
harness prints `P3A_RETRIEVAL method=neural status=BLOCKED reason=no_BEACON_EMBED_URL`.
It will run automatically if `BEACON_EMBED_URL` points at a live sidecar.

## 2. Method and controls

- Fixture: `reports/phase3/heldout/retrieval_context_v1.json` (locked, hash
  `FADF313E…114A`), 12 topics × 4 queries = 48 queries.
- Memories per topic: `T-primary`, `T-duplicate` (share finding `T-primary`),
  `T-support`, `T-decoy`. Ranking gold = `{T-primary, T-duplicate}`.
- Identical fixture, memories, scope (`project_id = phase3-heldout-retrieval`), and gold
  labels for every method. `Top-5`, `Limit=5`, `CandidateK=50`.
- **No weight tuning on the locked set.** Weights were fixed before running
  (hashed-semantic: `LexicalWeight=0, SemanticWeight=1`; hybrid: defaults `1/1`).
- Original lexical arm was compiled from `99c70908` using the **same** harness source.

## 3. Results (Recall@1 / Recall@5 / MRR / NDCG@5)

### Aggregate (n = 48)

| Method | R@1 | R@5 | MRR | NDCG@5 | p50 µs | p95 µs |
|---|---:|---:|---:|---:|---:|---:|
| Original lexical (99c70908) | 0.1667 | 0.1667 | 0.1667 | 0.1344 | 544 | 983 |
| V6 lexical | 0.1667 | 0.1667 | 0.1667 | 0.1344 | 571 | 1241 |
| V6 hashed semantic | 0.6875 | 0.8750 | 0.7635 | 0.7156 | 1267 | 1979 |
| V6 hybrid RRF | 0.6875 | 0.8750 | 0.7635 | 0.7156 | 1175 | 1689 |
| V6 hybrid ANN | 0.6875 | 0.8750 | 0.7635 | 0.7156 | 1142 | 1850 |
| V6 real neural (BGE-small) | — | — | — | — | — | **BLOCKED** |

### By subset (V6 lexical vs hashed semantic)

| Subset | n | Lexical R@1 | Lexical R@5 | Semantic R@1 | Semantic R@5 |
|---|---:|---:|---:|---:|---:|
| identifier | 12 | 0.6667 | 0.6667 | 1.0000 | 1.0000 |
| paraphrase | 12 | 0.0000 | 0.0000 | 0.5000 | 0.7500 |
| synonym | 12 | 0.0000 | 0.0000 | 0.6667 | 0.7500 |
| hard_negative | 12 | 0.0000 | 0.0000 | 0.5833 | 1.0000 |

## 4. Interpretation

- **V6 lexical == original lexical** on quality (0.1667 R@5). Both apply the same
  AND-substring predicate (`matchesText`); V6 only changes *how* it is executed. No
  ranking regression, but the predicate is brittle on natural-language queries: a single
  out-of-vocabulary word (e.g. "remediation" in `AUTH-401-EXP remediation`) yields an
  empty result. R@5 == R@1 because when all terms match, the sole matching memory ranks
  first.
- **Hashed semantic is materially better** (R@5 0.875 vs 0.167; R@1 0.6875 vs 0.1667),
  and it fully recovers identifier queries. It is still weak on paraphrase/synonym — the
  documented limitation of signed feature hashing (it captures morphology, not synonymy).
- **Hybrid RRF == semantic** on this corpus: with 48 short memories the lexical
  candidate list adds nothing the semantic list lacks, so RRF fusion does not move the
  metric. **Hybrid ANN == hybrid exact** here (48 vectors; ANN recall is exact at this
  scale).
- **Neural cannot be compared.** The plan's gate — "neural R@5 must match or exceed
  hashed and lexical" — is **NOT TESTED**, so neural/hybrid stay opt-in.

## 5. Reproduce

```
cd cli/beacon
BEACON_PHASE3_RETRIEVAL=1 go test ./internal/learning/ -run TestPhase3ARetrieval -v -count=1
# baseline (same harness, baseline worktree at 99c70908):
BEACON_PHASE3_RETRIEVAL=1 \
BEACON_PHASE3_RETRIEVAL_FIXTURE=<repo>/reports/phase3/heldout/retrieval_context_v1.json \
  go test ./internal/learning/ -run TestPhase3ARetrievalLexical -v -count=1
# neural (on the Mac, with sidecar):
BEACON_EMBED_URL=http://127.0.0.1:8077 BEACON_PHASE3_RETRIEVAL=1 \
  go test ./internal/learning/ -run TestPhase3ARetrieval -v -count=1
```

Raw evidence: `RAW_EVIDENCE/retrieval_v6.txt`, `RAW_EVIDENCE/retrieval_baseline.txt`.
