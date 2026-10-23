# Beacon V7 hardening — Phase A root-cause audit and profile

Status: **Phase A audit and optimization design complete; implementation not started.**
Audited source: `7b605aa84f0f570ba265fbda17fe6d99e927b10e` on isolated branch
`codex/beacon-v7-hardening`. The original, V6 integration, Phase 5, and V7
development refs remain unchanged. This document uses only local WSL2 data.

## Provenance and limits

| Reference | Verified commit | Role |
|---|---|---|
| Original | `99c709081ad49da1c5ade205929b61b19360ed9b` | Immutable comparator |
| V6 integration | `209a987bc96c29b5e5d811a794d0357f20214463` | Immutable comparator |
| Phase 5 | `02e34f98dec1c5d34a2f0d52904969edc39e389f` | Frozen evaluation evidence |
| V7 parent | `7b605aa84f0f570ba265fbda17fe6d99e927b10e` | Audited code |

The 56-task fixture was locked for the prior V7 pilot, but is **historical
development data now**. Its seven repeated mechanism families do not represent
56 independent real workflows. This audit reuses it only to measure the same
retrieval commands; it does not call a model, change the fixture, or claim a new
quality win. The WSL2 host has eight visible CPUs, 7.6 GiB RAM, Linux/amd64,
Go 1.27, and an i5-1155G7. See `raw/profile/environment.txt` and
`raw/profile/cold_cli.json` for exact machine, source, binary, and fixture hashes.

## Actual execution graph

```text
CLI memory list --question-aware [--semantic]
  memoryQuery -> project/path resolution -> runMemoryList
  -> Open or OpenSemantic(hashed embedder) -> SearchQuestion
      scopeQuery -> ListMemories
        db/openLocked -> ensureSchemaOnce (per-process path cache)
        -> searchMemories
          memorySearchTerms -> ensureIndexReady (maybe full FTS rebuild)
          -> queryIsSelective / bounded posting-list probe
          -> currentGeneration -> searchCacheGet
          -> fetchIndexedCandidates -> JSON decode -> exact original matcher
          -> sort/paginate -> searchCachePut
          fallback: original scoped scan on short term/index failure/overflow
      -> reject instruction-bearing lexical notes
      -> SelectTrustContext (injection/source/supersession)
      -> analyzeQuestion -> direct evidence / bridge classification
      if no direct support and embedder configured:
        ensureQuestionEmbeddings (project count; optional read/backfill/commit)
        -> embed query -> exact vector scan, or opt-in ANN lookup
        -> bridge-owner map -> direct/slot/link validation
      -> SelectTrustContext (injection/source/supersession/conflict)
      -> whole-memory item/token budget -> sufficiency signal
  -> Close -> JSON encode / text output -> caller/agent
```

The production call site is `cli/beacon/cmd/memory.go:278`. Core source is
`internal/learning/search_question.go:33`, `store.go:102,635,736`,
`search.go:480`, `search_question_index.go:13`, `search_semantic.go:115`,
`context_select.go:74`, and `search_cache.go:79`. The default CLI path without
`--question-aware` still uses `ListMemories`; the V6 hybrid/RRF API remains for
compatibility but is bypassed by V7 question-aware search.

### Work and correctness at each boundary

| Boundary | Queries, copies, computation, and ordering | Lock/cache/error behavior |
|---|---|---|
| Scope | `scopeQuery` resolves project path via store lookup if no ID; CLI's `memoryQuery` establishes scope. | An absent scoped project/question/query causes explicit error. Scope is also applied to SQL and semantic query. |
| Pool/schema | `db()` lazily opens SQLite and calls `ensureSchemaOnce`; schema has authoritative and derived tables; WAL is initialized. | Store mutex protects pool; global schema mutex/cache protects same-process initialization. New CLI process loses that cache. `busy_timeout=30000`, immediate writer transaction. |
| Lexical index | `ensureIndexReady` may rebuild all FTS rows in a transaction. `queryIsSelective` does one `memory_trigram_df` SQL statement per term; fallback probes 257 postings. `fetchIndexedCandidates` joins FTS to memories and JSON-decodes up to 256 rows. | Global rebuild mutex; in-process ready cache. Errors/unselective terms/short terms fall back to authoritative scan. Rebuild on first use can write and delay a read. |
| Lexical cache | `currentGeneration` reads durable metadata; key includes path, project, kind, raw query, limit, page and ranking version. Hit returns cached memory values. Miss runs FTS, original matcher, stable newest-first sort, pagination, then put. | Process-local 32 MiB LRU under mutex; durable generation changes on writes and is checked before serving entries. A process restart has no warm cache. |
| First trust pass | Separate `hasInjection` scan then `SelectTrustContext`; source heuristic, injection handling and `resolveSupersessions` create maps, term sets and candidate pairs. | Trust applies before relevance. Body text and `source:` tags are not authenticated provenance. No database query here. |
| Relevance | `analyzeQuestion` lowercases and tokenizes question/query; `direct`, `primaryMatch`, `slotMatch`, `definitive` repeatedly build token maps from title/body/applicability. | Heuristic slot/primary interpretation; errors are false support, false abstention or contamination. This is a quality risk, not merely a performance cost. |
| Semantic decision | Runs only with no direct lexical support and an explicitly configured embedder. CLI `--semantic` uses hashed vectors; neural is not the CLI default. | Route is emitted. No embedder means lexical insufficiency is preserved. |
| Backfill | `ensureQuestionEmbeddings` runs a project-scoped `COUNT` join. If missing, reads and JSON-decodes every missing row (maximum 1,000), then starts one transaction; each row checks prior hash/model and upserts embedding. | First request can hold a writer transaction; above cap returns an explicit error. Model/dimension mismatch triggers backfill. A lexical writer invalidates a derived vector. |
| Exact vectors | `semanticSearchExact` joins all project vectors with `memory_json`; scans BLOB, dimension and JSON for every row; decodes each vector, scores cosine, retains top K, sorts winners. | O(project vectors × dimension), with per-row BLOB/float copies. ANN is optional and has separate correctness/quality gates. Errors return rather than silently fabricating support. |
| Bridge | `bridgeOwners` maps each term to memory IDs; each hit's terms are recomputed for direct, slot and links. Only a unique discriminative link may support the second hop. | Similarity alone is not factual support. Competing shared terms can still expose heuristic failure; needs diverse tests. |
| Final trust/budget | A second `SelectTrustContext` runs only on selected evidence with conflict enabled. Signal IDs are sorted. `selectWholeQuestionContext` counts title/body tokens and keeps whole memories, never a misleading fragment. | Sufficient only if trusted selected evidence survives, has no unresolved conflict and fits budget. JSON encoding occurs after Store closure in CLI. |
| Writer/invalidation | `PutMemory` marshals JSON, `BEGIN IMMEDIATE`, upserts authoritative row, updates FTS if healthy, increments durable generation, upserts or invalidates embedding, then commits. | SQLite serializes writers; 30 s busy timeout bounds wait but does not reduce p99. Index errors preserve authoritative write and mark derived index unhealthy. Crash hooks surround transaction phases. |

**Serialization points:** each CLI call starts a process, parses flags, opens
SQLite, decodes `memory_json`, and JSON-encodes the final result. The Go
`database/sql` pool may reuse connections only within a process. On-demand
backfill and FTS rebuild use SQLite write transactions; concurrent writes
serialize at SQLite even when Go mutex profiles are quiet.

## Measured latency and allocation

### Paired cold-process CLI, same 56 stores and query distribution

The audit-only `cold_cli.py` seeded each existing fixture task once on WSL2,
ran original then V7 (preserving the prior validation order), and repeated each
arm against that same store with randomized repeat order. Each invocation was
a new process. The first V7 call may initialize FTS and vectors; the repeat
uses persistent derived state but **not** an in-process result cache. No model
call is included. Raw: `raw/profile/cold_cli.json`; summarizer and output:
`raw/profile/summarize_cold.py`, `cold_cli_summary_v2.txt`.

| Pass | Arm | n | Mean | p50 | p95 | p99 |
|---|---|---:|---:|---:|---:|---:|
| First | Original | 56 | 14.66 ms | 13.47 ms | 19.32 ms | 56.29 ms |
| First | V7 | 56 | 43.71 ms | 40.81 ms | 51.85 ms | 135.34 ms |
| Repeat | Original | 56 | 13.81 ms | 13.33 ms | 17.51 ms | 24.46 ms |
| Repeat | V7 | 56 | 14.85 ms | 14.43 ms | 24.13 ms | 26.06 ms |

First-pass paired V7-minus-original median was +27.02 ms, mean +29.05 ms;
V7 was slower on 55/56 tasks. Repeat paired median was +0.72 ms, mean
+1.04 ms; V7 was slower on 41/56. First V7 lexical-only-route tasks averaged
42.30 ms (40 tasks), while first semantic-route tasks averaged 47.23 ms
(16 tasks). Repeat means were 15.04 and 14.39 ms respectively. The route
classification is V7's emitted route, not a model-quality result.

The no-database `beacon version` control, randomized across 56 invocations per
binary, measured median process overhead of 9.10 ms original and 8.82 ms V7
(`startup_cli.py`/`startup_cli.json`). It does not measure exact startup within
`memory list`, but rules out a large binary-startup difference. A separate
audit-only internal stage probe (`cold_stages.py`, `phase_a_stages_test.go`,
`cold_stages.json`) seeded one fresh original-style database per mechanism
family, then timed each production function sequentially. Across those seven
small stores, `db_open_schema` took 1.3–2.8 ms; **`fts_ready` took 17.0–44.4
ms**; lexical search after that took 0.54–1.56 ms. In the chain case, embedding
backfill took 9.84 ms and exact vector search 0.47 ms. In the uncertain case,
the full question call took 8.41 ms because it performed its own unprepared
semantic work. Warm full question calls on the other six preprepared paths
took 0.18–0.85 ms, and JSON encoding took 0.12–0.37 ms. This is a seven-case
stage probe, not a population p95. It identifies synchronous first-use FTS
index build as the primary cold mechanism on these original-style stores,
with vector backfill as an additional semantic mechanism. The repeat CLI
result shows this particular small fixture has little steady cold-process
difference, but does not prove parity at 1K–1M scale or on diverse questions.

### Warm in-process Go microbenchmarks

The audit-only `phase_a_profile_test.go` was copied into a disposable WSL2
checkout; it is **not** production code. Three unprofiled runs on the same
1,000-record seeded dataset showed direct `SearchQuestion` at 78.2/97.4/110.4
µs, 7,992 B and 117 allocations per operation. `ListMemories` with the same
query and result limit was 36.4/41.3/44.8 µs, 1,560 B and 50 allocations.
This is a paired-path overhead test, not original-vs-V7 quality or throughput.
At 200 records, the separate semantic workload was 2.81/2.93/3.05 ms, about
1.55 MiB and 4,087–4,094 allocations per operation. Workloads and sizes differ;
those figures must not be divided into one another as a speedup ratio. The
first semantic call on 201 records took 22.3/25.1/27.2 ms across fresh test
stores, then 2.3–4.0 ms on each immediate second call. The first call includes
lazy pool work and missing-vector backfill, so the exact backfill share is not
isolated. Raw outputs: `benchmark.log` and `semantic_first.log`.

### CPU, allocation, block, mutex, trace

`run_phase_a_profile.sh` produced Go CPU, heap-allocation, block, mutex and
execution traces for direct and semantic paths. Text pprof views are in
`raw/profile/*_top.txt`; binary `.pprof` and `.trace` files preserve the full
call graphs. Go benchmark setup is sampled by `-cpuprofile`; therefore the
derived `*_searchquestion_top.txt` files focus on the `SearchQuestion` stack
and exclude setup stacks. Percentages printed by pprof still use total-profile
denominators. A direct-path focused stack contains 11.69 s cumulative CPU
samples out of 16.93 s total, including SQL VM, tokenization and trust work.
Allocation attribution on that stack: `SelectTrustContext` 511 MB cumulative,
`significantTerms` 334 MB, `resolveSupersessions` 214 MB, and
`candidatePairs` 179 MB over the sampled run. These are overlapping inclusive
costs; do not sum them.

For semantic retrieval, `semanticSearchExact` accounts for 1,674 MB cumulative
allocation, about 91% of the total 1,845 MB profile. Three prominent flat
allocators are SQLite `columnBlob` (489 MB), `bytes.Clone` in row scanning
(479 MB), and `decodeVector` (470 MB). These represent repeated copies of
vectors. CPU includes the SQLite VM, vector decoding/cosine and JSON decoding.
The exact code selects `memory_json` for every candidate but unmarshals only
top-K rows; SQLite and `Rows.Scan` still materialize strings/BLOBs for all.

The single-client block profiles are dominated by Go benchmark/runtime and
connection-cleaner waiting. Mutex profiles show no material application-level
contention in this workload. They **cannot** clear the known 16-client writer
tail; Phase D needs sustained multi-client transaction timing. The traces are
retained for deeper timeline analysis if a specific optimization requires it.

## Root causes ranked by evidence and impact

| Rank | Weakness and causal evidence | Severity / impact | Smallest candidate change; required proof |
|---:|---|---|---|
| 1 | First V7 call pays ~29 ms paired mean over original on tiny seeded stores; repeat gap ~1 ms. Seven fresh stage probes place FTS readiness at 17–44 ms and one semantic backfill at 9.84 ms. | High user-visible cold latency; possible lock interference. | Move **bounded** derived-state preparation outside latency-sensitive request when practical; retain exact scan fallback and explicit incomplete-vector signal. Rerun paired cold paths and crash tests. |
| 2 | Exact semantic path allocates ~1.55 MiB/op at 200 rows; 91% of profiled alloc bytes inclusive in `semanticSearchExact`; three vector-copy sites dominate. | High at larger stores; potential GC and p99. | Scan vector/id only, score from encoded BLOB without a float-slice copy, hydrate only top K, preserving scope/order/exact cosine. Compare correctness, allocation and p95 against untouched code. No ANN default switch. |
| 3 | Direct question path adds ~56 µs median and ~6.4 KiB/op over same-store `ListMemories`; first and second trust passes plus repeated term maps are visible in allocation profile. | Medium warm-path cost; correctness-sensitive. | Cache term sets **within one request** or consolidate safe trust work; preserve mandatory trust on every route. Verify injection, supersession, conflict, scope, budget and near-miss cases before claiming a win. |
| 4 | A cache hit still needs selectivity SQL plus durable generation SQL before result lookup. | Medium at high QPS, but needed for cross-process invalidation. | Measure query count/timing, then consider checking durable generation before selectivity only if index-health and cache semantics remain sound. Never bypass scope/trust or return stale data. |
| 5 | Heuristic slot/primary parsing and unique bridge-term rule can omit or contaminate evidence; pilot families are narrow. | High task-success risk, contribution unproven beyond synthetic pilot. | Build disjoint development adversarial cases and untouched real-workflow holdout. Favor generalized parsing and evidence-support checks; no fixture-specific rule. Evaluate false abstention and unsupported answer jointly. |
| 6 | Prior five-second 16-client probe: writer p99 742 ms V7 vs 635 ms Phase 5, zero reported errors. `BEGIN IMMEDIATE`, FTS, generation and vector work occur in one SQLite transaction. | Release blocker, cause unresolved. | Instrument begin wait, transaction body, commit and WAL checkpoint under sustained mixed load before tuning. Keep durability and SQLite single-writer semantics explicit. |

## Complexity decisions

The classifications below are design decisions, **not** claims that any
component has already been removed or optimized.

| Component | Decision | Evidence and reason |
|---|---|---|
| Authoritative scoped SQLite store, approval, supersession field, crash hooks | KEEP | Foundation for durable and isolated memories; removal breaks correctness. |
| FTS5 lexical index plus exact original matcher and scan fallback | KEEP / OPTIMIZE | Indexed speed at scale was established in earlier phases; first-use work and query count now need attention. Preserve parity. |
| Generation-checked bounded query cache | KEEP / OPTIMIZE | Warm benefit, cross-process invalidation; audit stampedes, eviction, mutable result risk and mixed writes. |
| Question-aware direct evidence and explicit insufficiency | KEEP / OPTIMIZE | First locked pilot improved two of seven mechanism families; parsing generalization remains unproven. |
| Source/trust/injection checks | KEEP / OPTIMIZE | Mandatory security boundary; current `source:` is unauthenticated and strict instruction drop can false reject. |
| Supersession and relevant-conflict detection | KEEP / CONSOLIDATE | Needed for timeline correctness, but candidate-pair/term construction is repeated. Avoid always-on irrelevant conflict work. |
| Whole-memory item/token budget | KEEP | Preserves qualifiers and signals when support does not fit; cheap relative to SQL/vector work. |
| Hashed semantic expansion and exact search | MAKE OPTIONAL / OPTIMIZE | Two-hop benefit on one synthetic family; costly first use and per-row copies. CLI already opt-in. |
| Neural embedder and HNSW ANN | MAKE OPTIONAL | No independent agent-level gain or operational gate passed; retain API/compatibility. |
| V6 hybrid RRF and older selector APIs | CONSOLIDATE LATER | V7 does not call RRF; retain compatibility until usage inventory and migration plan prove safe removal. |
| Extra always-on model call or another ranker | REMOVE FROM PROPOSAL | No measured need; adds latency and operational complexity. None is currently on V7 default path. |

## Optimization design gate before Phase B implementation

1. Freeze this source/evidence. The seven-case stage probe now isolates SQLite
   open, FTS readiness, lexical search, backfill, exact scan and JSON on
   original-style pre-index stores. Before a cold-path code change, extend
   opt-in timing to generation/cache and transaction waits at realistic sizes;
   never expose instrumentation overhead in default agent context.
2. Treat correctness as a gate for each local optimization: exact result IDs,
   project isolation, fresh generation after cross-process writes, injection
   exclusion, conflict/temporal handling, and whole-memory budget behavior.
3. Implement the smallest high-value change first, one focused commit at a
   time. A candidate change must beat the untouched V7 parent under matched
   source, data, hardware and query mix. Do not trade a small microbenchmark
   gain for worse agent outcomes or writer p99.
4. Reserve historical Phase 5 and the 56-task pilot for regression only.
   Create development adversarial cases separately and lock an independent
   real-workflow holdout before final model comparison. Keep neural and ANN
   opt-in through their separate gates.

**Phase A verdict:** root causes and ranked change design are recorded. The
first-call latency regression and its dominant FTS rebuild mechanism are
measured on fresh original-style stores; semantic backfill adds a separate
first-use cost. Realistic-size stage timings remain a Phase C gate. V7 has not
yet demonstrated overall speed superiority or release readiness.
