# Beacon V6 integration verification — 22 October 2026

## Provenance and result

The isolated local branch `integration/beacon-v6-final` merges Phase 1
`db32a67bed95e3dc5791ed6bb5e162fefe8ff8db` and Phase 2
`05bc122bba800b76d6fb3641e6c6767560ff2a4d`. Their exact common
ancestor is `85e6ea1cbb25ac193bd902966aa5889bf174dfa3`; the original
pre-V6 baseline is `99c709081ad49da1c5ade205929b61b19360ed9b`.
The two-parent code merge is `394e631e7ddd98347795253b546ced527b76cf8e`
(parents Phase 1, then Phase 2). Phase 2 was recovered from its Mac-only Git
history through a verified bundle. No remote push or PR was made.

The integrated code builds and passes the repository-wide tests on macOS
Apple M5/arm64 and native WSL2/Linux amd64 with Go 1.27.3. All focused
correctness checks described below passed. No integration-specific defect was
found. This report and its evidence logs are the only post-merge changes.

## Conflict review

Git reported three textual conflicts:

| File | Resolution |
|---|---|
| `cli/beacon/internal/learning/search_ann.go` | Retained Phase 1 rejection of non-finite, zero, and dimension-mismatched vectors, plus Phase 2 seeded RNG and `EfConstruction` settings. |
| `cli/beacon/internal/learning/search_trust.go` | Retained Phase 2 precomputed polarity/terms and overlap-ratio filter, plus Phase 1's distinct-explicit-version guard. Both labeled trust matrices pass. |
| `cli/beacon/internal/learning/store.go` | Retained Phase 1's per-store crash checkpoints and Phase 2's process fault points at each write boundary. The 30-second SQLite busy timeout remains. Both crash harnesses pass. |

`search.go` migration/index fault points, `search_context.go` token budgeting,
the optimized HNSW graph, and all Phase 1/Phase 2 tests and reports merged
without textual conflict. No test was deleted or deduplicated. The combined
tree passes `gofmt` and `git diff --check`.

## Validation

The Mac checkout and native LF-normalized WSL clone were at the exact merge
SHA. From `cli/beacon`, both ran `make build-hooks-current`, `go build ./...`,
`go vet ./...`, and `go test -p 2 -timeout 30m -count=1 ./...`: **PASS**.
The Python sidecar unit test also passed. The initial WSL run directly from a
Windows-mounted worktree failed unrelated Vector template assertions because
the files had CRLF line endings; the native WSL checkout of the same commit
passed that package and the full suite. The Mac full suite passed twice.

| Area | Combined-code result |
|---|---|
| Race detector | Mac learning and HNSW: PASS (`go test -race -p 2 -timeout 30m -count=1`, 343.858 s for learning). WSL learning and HNSW: PASS (718.066 s for learning). |
| Differential search | 10,000 randomized queries and edge cases: PASS. |
| Crash/durability | Phase 1 write-boundary recovery and Phase 2 write, migration, index-rebuild, and supersession process-crash tests: PASS. |
| SQLite and migrations | Bounded write contention, original-schema upgrade, index deletion/rebuild, and interrupted-startup recovery: PASS. |
| Trust/security | Phase 1 50 labeled scenarios: TP 10, FP 0, FN 0 on 30 pairs. Phase 2 65-case adversarial matrix: TP 25, FP 0, FN 8, TN 32; precision 1.000, recall 0.758, 37 cross-talk pairs. Cross-scope, unapproved-evidence, hostile-content, and retrieval smoke tests: PASS. |
| Context/ranking | Actual body truncation, token budgeting, Phase 2 ranking alternatives, and evidence-selection strategies: PASS. The merged Phase 2 budget harness retains 106/144 findings versus 100/144 on its Phase 2 parent. |
| ANN | Invalid-input rejection, HNSW determinism, package invariants, and 12-cell Phase 1 matrix: PASS. At 10K/M64, Recall@1 and Recall@10 were both 1.000 on the seeded 64-query synthetic matrix. No 1M run. |
| Concurrency | Phase 1 20-case read/write/mixed/supersede/rebuild matrix: PASS, zero errors. Phase 2 bounded 5-second runs at 1, 4, 8, and 16 clients: PASS, zero errors, busy events, or stale reads. |
| Real neural sidecar | Local cached `BAAI/bge-small-en-v1.5`, MPS, 384 dimensions: both Phase 1 quality tests PASS; finite and normalized 32-item batches. On the 40-case extended set, neural R@5 was 1.0 paraphrase, 0.6 synonym, 1.0 hard negative, and 1.0 identifier. |

Logs and measured outputs are under `reports/integration/raw/`. These checks
use synthetic fixtures and process-crash injection. They do not establish
power-loss durability or real-world prompt-injection resistance.

**Phase 2 report discrepancy.** `reports/phase2/TOKEN_RANKING_EVALUATION.md`
claims held-out Recall@1 of 0.812 for hashed semantic and 0.729 for hybrid
RRF. Its own committed raw log instead says 0.208 and 0.167. Fresh tests on
the untouched Phase 2 parent and the merged tree reproduce the raw values
exactly. The relative ranking remains semantic above RRF, but the report's
absolute table is incorrect and must not be used as an evaluation baseline.
The merged token-budget path retains 106/144 relevant findings versus the
Phase 2 parent's 100/144, with 3,840 versus 3,535 tokens. Against unbudgeted
top-five selection (109/144, 5,674 tokens), the merge loses 3 findings (2.8%
of those retained by top-five), improving on Phase 2's 9-finding loss. This
is an inherited reporting error and a merged behavior improvement, not a
ranking regression.

## Matched performance, same Apple M5

Original Beacon `99c70908` and merged V6 used the same existing
`BenchmarkPhase1Lexical` source, Go 1.27.3, 30 timed iterations per cell,
sequentially on the same Mac. Setup was excluded from timing.

| Corpus/query | Original | Integrated | Change |
|---|---:|---:|---:|
| 1K/no match | 1.857 ms/op | 0.282 ms/op | 6.58× faster |
| 10K/rare | 1.148 ms/op | 0.318 ms/op | 3.61× faster |
| 10K/no match | 16.477 ms/op | 0.331 ms/op | 49.78× faster |

The original baseline predates the ANN implementation, so HNSW was compared
against the common V6 ancestor `85e6ea1c` with the same Phase 2 timing harness,
synthetic vectors, `M=64`, and Mac. This comparison is sequential and uses one
build per size; it is not a statistical latency distribution.

| Vectors | Pre-optimization build | Integrated build | Speedup | Peak RSS |
|---:|---:|---:|---:|---:|
| 1K | 962 ms | 194 ms | 4.96× | 32 to 20 MB |
| 10K | 20,804 ms | 5,826 ms | 3.57× | 125 to 54 MB |

## Remaining defects and Phase 3 blockers

No integration regression remains in the tested envelope. The following
existing limitations need explicit Phase 3 decisions or evidence:

1. **Real-neural held-out ranking:** the merged real sidecar passes the Phase 1
   17/40-case checks, but Phase 2's 96-query ranking/token evaluation still
   uses a hashed embedder. Re-run that held-out evaluation with BGE-small before
   making neural or semantic-heavy ranking the default. Synonym R@5 is only
   0.6 on the small neural set.
2. **Sustained multi-writer behavior:** short runs had no errors, but 16-client
   write p99 was about 0.75 s and Phase 2's 15-minute 8-client soak reported
   read/write p99 about 0.65/0.75 s under forced index rebuilds. Set an
   operational writer budget or address connection churn and blocking rebuilds
   before claiming low tail latency under that workload.
3. **Contradiction recall and cross-talk:** the adversarial matrix remains at
   0.758 recall with 37 cross-talk pairs. Extend cross-kind detection and
   revalidate thresholds on held-out cases before using it for automatic
   decisions. The current API only flags cases for human review.
4. **Evidence loss under token budgets:** the merged held-out budget selector
   still loses 3 of the 109 findings retained by unbudgeted top-five selection
   (2.8%). Keep budgeting opt-in or adopt evidence-preserving deduplication,
   then measure the tradeoff with real tasks.
5. **Low-`M` ANN behavior:** retain `M=64` as the verified default. Diversity
   neighbor selection and a broader low-`M` recall sweep are future work; no
   1M benchmark was attempted.
6. **Agent/security generalization:** the storage trust boundary held for the
   synthetic hostile-content tests, but agent-level prompt injection and a
   statistically meaningful original-versus-V6 task-success advantage are not
   established. The local three-task agent harness is available for Phase 3.
7. **Optional build reproducibility:** HNSW randomness is seedable for tests,
   but default builds are not bit-reproducible. Seed the default if exact graph
   reproducibility is a Phase 3 requirement.
8. **Evaluation report repair:** correct Phase 2's absolute ranking table and
   establish a locked held-out baseline before using it to decide defaults.

Default CLI/MCP/dashboard retrieval still uses the lexical, offline path.
Neural search and ANN are opt-in. No paid compute or 1M benchmark was used.

## Agent-evaluation readiness

The existing local three-task coding-agent harness, seed binary, Mac Ollama
`qwen2.5-coder:7b`, and both original and integrated CLIs ran successfully.
Results were 1/3 successful with no memory, 2/3 with original Beacon, and 2/3
with integrated V6. Both memory arms retrieved identical IDs in all three
tasks; the slug task failed in every arm. Retrieval was 9.60–12.04 ms. The
CLI, seeding, retrieval, model calls, tool loop, and objective task tests are
ready for larger evaluation, but this smoke run shows no V6 advantage over
original Beacon. Phase 3 needs larger held-out tasks, multiple runs, and a
real-neural ranking comparison before making a quality claim.
