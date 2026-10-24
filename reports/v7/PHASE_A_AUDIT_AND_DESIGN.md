# Beacon V7 — Phase A architecture audit and design

Status: design completed before V7 implementation. Source commits: original
`99c709081ad49da1c5ade205929b61b19360ed9b`, V6 integration
`209a987bc96c29b5e5d811a794d0357f20214463`, Phase 4D
`7c7e24933dbef69fe47599c35c211ee24f383a1d`, Phase 5
`02e34f98dec1c5d34a2f0d52904969edc39e389f`.

## What the code actually executes

```text
Original memory list
  scopeQuery -> SQLite approved, non-superseded rows -> AND-substring matcher
  -> newest-first pagination -> CLI / agent

V6 default memory list
  scopeQuery -> adaptive FTS5 or original scan -> generation-checked cache
  -> exact original matcher -> newest-first pagination -> CLI / agent

V6 opt-in --semantic --intelligent [--sufficiency]
  SearchHybrid: lexical candidates + embeddings (exact or optional ANN)
  -> reciprocal-rank fusion -> SelectTrustContext:
     injection handling -> source heuristics -> supersession -> pairwise conflict
     -> surviving evidence -> optional coverage check -> item budget -> CLI / agent

Other optional API: SearchHybrid -> SelectContext (token/body budget)
Writer: approved SQLite memories -> transactional FTS5/index generation;
        embeddings and ANN are derived/rebuildable state.
```

The CLI call site is `cli/beacon/cmd/memory.go:runMemoryList`; core paths are
`learning/store.go:ListMemories`, `search.go:searchMemories`,
`search_semantic.go:SearchHybrid`, `context_select.go:SelectTrustContext`,
and `search_context.go:SelectContext`. No production call site currently supplies
the *agent question* separately from the terse retrieval query. The Phase 5
selection harness also passes `retrieval_query` to both retrieval and sufficiency.
Thus a selector cannot know the requested fact, answer type, or exclusions.

## Component decisions

| Component | Purpose and dependency | Measured benefit / runtime cost | Failure mode | V7 decision |
|---|---|---|---|---|
| Authoritative SQLite store, approval and project scope | Durable facts; all paths depend on it | Crash suites: zero acknowledged-write loss; 16-client writer p99 744–795 ms | Single-writer contention | Keep; investigate 16-client separately |
| Adaptive FTS5 and original matcher | Indexed retrieval with exact scan parity | 100K selective FTS5 p50 0.026 ms in Phase 5 synthetic scale test; original BM25 comparator 49.2 ms | Index corruption, query nonselectivity | Keep; preserve scan fallback and parity |
| Generation-checked 32 MiB cache | Fast repeated indexed queries | Warm-query benefit in earlier phases; generation invalidates cross-process writes | Mutation of cached objects or generation bug | Keep; audit with race/update tests |
| Hashed embeddings, exact vector search | Optional paraphrase candidates | B3 multi-hop 0/20 original vs 8/20 V6 arm; ~1 ms retrieval reported | Adds irrelevant candidates; exact scan cost at scale | Optional, after lexical sufficiency |
| Neural sidecar and ANN | Optional real semantic recall and large vector stores | ANN recall/build measured; no agent-task win shown for neural; ~43 ms neural query over LAN | Extra operational dependency, recall/scale uncertainty | Optional only; no default route |
| RRF with two weight knobs | Merge lexical and semantic ranks | No isolated end-to-end benefit established | Duplicate scoring and weak exact-match protection | Bypass in V7 path; retain API for compatibility |
| Trust and injection selection | Remove untrusted or hostile notes | B5 20/20 V6 intelligent vs 14/20 original in Phase 5 | Source tags and body words are not authenticated provenance | Mandatory on V7 path; preserve current behavior while separately fixing provenance |
| Supersession / contradiction pass | Remove obsolete facts, expose unresolved pairs | History tests pass; B1 agent task 8/20 in all arms | False conflict or costly pair discovery | Keep supersession; run conflict only among selected relevant candidates |
| Query-term sufficiency | Detect missing support | B2 V6 16/20 vs original 20/20 | Terse query coverage can bless a non-answer | Replace in V7 path with question-aware support; keep old API |
| Item cap and token budget | Bound context | V6 149 vs original 131 prompt tokens per task | Truncation or omission of qualifiers | One final selection pass after trust; retain provenance |

The Phase 4D selector's ~36x speedup at 1,000 candidates is a microbenchmark;
Phase 5's K=8 agent path is dominated by model time. It is not a reason to
rewrite the indexed selector or to make semantic retrieval mandatory.

## Failure evidence and causal limits

The frozen Phase 5 suite scored original 229/300, V6 intelligent 224/300,
V6 labelled “semantic” 209/300, oracle 246/300. The 22 original-correct /
V6-wrong cases comprise 18 A3 knowledge-transfer cases and four B2 ambiguity
cases. In `a3-41`, original returns only the Go-service convention and answers
correctly; V6 returns that convention plus a verified billing-team note, then
abstains. In `b2-124`, original returns no note and abstains; V6 passes through
a “sometimes uses canary” hint and guesses canary. Both traces show that recall
is not the missing piece: context precision and support are.

The 24 shared B1/B3 failures are model/reasoning failures under the current
prompt, not proven retrieval failures. Of the oracle's 54 failures, 20 are the
invalid A1 task, 12 B1, 13 B3, seven C1, one A2 and one C5. Adding retrieval
cannot by itself cure these. The current B1 contradiction detector also has
0.50 default recall on the locked Phase 3 set; cross-kind detection is opt-in.

**Measurement caveat:** `HybridOptions.withDefaults` converts zero weights to
one. The Phase 5 C/D selection call passes `LexicalWeight:0`, apparently to
request semantic-only retrieval, but executes lexical + semantic RRF. The
stored results and reports retain their provenance; their arm labels must not
be used to claim a clean semantic-only ablation. Also the selection dump does
not record whether `BEACON_EMBED_URL` succeeded or the hashed fallback ran.
Phase 5 reports state hashed; model provenance needs explicit logging in V7.
The 300 tasks are historical diagnostics after this audit, never a fresh holdout.
The seeded templates repeat mechanisms, so 300 task rows are not 300 independent
real-world workflows.

## V7 architecture hypothesis

1. Keep `ListMemories` and the existing CLI defaults byte-compatible. New V7
   entry point is opt-in until paired agent quality and operational gates pass.
2. Accept both `retrieval query` and full `question`. Lexical search first, using
   the existing scoped store, index, cache and exact matcher. Apply trust and
   injection rules to **every** candidate path before giving evidence to an agent.
3. Use a deterministic question-support decision that asks whether selected
   evidence covers the requested entity and fact. A shared subject alone is
   insufficient. “Sometimes”/hypothetical notes cannot establish a definite
   operational choice. When support is unclear, return uncertainty rather than
   manufacturing a value.
4. Only when lexical evidence is insufficient and an embedder is explicitly
   configured, search semantic candidates. Keep direct lexical support in rank
   order; add semantic candidates only if they support the question or bridge a
   needed relation to already selected evidence. Never let semantic similarity
   alone displace a direct exact match.
5. Detect conflicts only among relevant surviving evidence. Preserve both
   sides, time and provenance when unresolved. One final item/token budget pass.
6. Avoid a new model call for routing. Keep ANN separately opt-in inside the
   conditional semantic branch. Expose route, candidate counts and reasons in
   diagnostics without putting implementation internals in agent context.

The question-support rule is a hypothesis. First validate it on **new**
development cases with different entities and histories. Then freeze an
independent holdout before model comparison. Historical A3/B2 cases are
regressions only, never tuning targets or evidence of V7 superiority.

## Controlled implementation and evaluation order

1. Add an explicit V7 entry point and isolated tests for support, ambiguity,
   exact-fact preservation, trust, project scope, supersession and budget. No
   changes to approved-memory schema or existing `ListMemories` behavior.
2. Compare original, V6, lexical+trust, question-aware, conditional semantic,
   and conditional conflict on new development data. Record selection IDs,
   model identity, route, timing, tokens and paired correctness.
3. Run the same configurations on an untouched independently locked agent-task
   set. Use one fixed local model/prompt, randomized arm order, paired analysis
   and confidence intervals. Report categories and every regression; do not
   claim superiority from the Phase 5 suite or repeated deterministic seeds.
4. Repeat build, vet, unit, race, crash, cache invalidation, trust and mixed
   read/write tests. Diagnose the known 16-client p99 failure separately.
5. Retain V7 opt-in if task success, security, latency or writer gates fail.
   No 1M benchmark or paid compute is required for this decision.

All work uses the existing Mac and WSL2 machines. Phase 5 fixtures, traces,
reports, original baseline and verified V6 branches are immutable references.
