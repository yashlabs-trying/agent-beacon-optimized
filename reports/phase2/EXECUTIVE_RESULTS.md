# Beacon V6 — Phase 2 Executive Results

## Scope and isolation

- Worktree: `/Users/<user>/beacon-phase2-opencode`
- Branch: `test/beacon-v6-phase2-opencode`
- Base SHA: `85e6ea1c` (Phase 1 verified V6; the latest committed V6 SHA at
  isolation time)
- Codex's worktree (`beacon-phase1-v6`, `beacon-phase1-baseline`) was never
  modified, reset or cleaned. No push, no PR. Budget: ₹0 (local only).

## Hardware

Apple M5, 10 logical CPUs, 16 GB unified memory, macOS 26.6.2 (25G83), arm64,
go1.27.3 darwin/arm64. **All results are macOS/Apple-Silicon.** No WSL2/Linux
run was performed in this session, and no neural-model inference was run here
(that is Codex's workstream), so no Linux or neural claim is made.

## Headline results

| Area | Result |
|---|---|
| HNSW build | **3.5–5.6× faster** at equal `M=64`; peak RSS ~2.8× lower; Recall@10 0.9949 @100K (unchanged) |
| Durability | **0 lost acknowledged writes, 0 spurious visible writes** across 10 crash points; integrity ok |
| Trust | Contradiction precision 1.000, recall 0.758 (misses are cross-kind by design); cross-talk 74→37 after fix |
| Security | No scope leak, no approval bypass, hostile content inert; 0 critical violations |
| Token budget | 1.61× token reduction **loses ~6% of distinct findings** (tradeoff); dedup is the only mild win |
| Ranking | RRF R@1 0.729 < semantic-only 0.812; semantic-heavy weight recovers to 0.812 |
| Concurrency | Race-free; 0 errors/busy at 1–32 clients; write tail degrades above ~8 writers |
| Scale | Max validated **100,000 memories**; bounded memory (~387 MB at 100K HNSW) |
| Integration | CLI/MCP/dashboard build and tests pass; offline lexical default preserved |

## Defects found and fixed

1. **HNSW build cost** — `maps.Keys` (51% of bytes) and neighbour-key sort (~32%
   of search CPU) dominated construction, plus interface-boxing allocations and
   repeated distance recomputation. Rewrote adjacency as slices with cached edge
   distances and a reusable search scratch. (commit `d15a8bca`)
2. **`detectContradictions` footgun** — zero options flagged every
   opposite-polarity pair; only `AnnotateTrust` guarded it. Hardened the helper
   and added an overlap-ratio rule that cut dense-corpus cross-talk 74→37 with
   no loss of true conflicts. (commit `ee7c71ac`)
3. **Crash-child kill race (test)** — the SIGKILL fallback could win the delivery
   race and report exit 137; accepted both. (commit `76109dc9`)
4. **Ranking default regression confirmed** — standard RRF underperforms
   semantic-only; added (not enabled) weightable RRF. (commit `2baa94f8`)

No data-corruption or lost-write defect was found. No scope or approval bypass
was found.

## Acceptance matrix

| Area | Required evidence | Status |
|---|---|---|
| HNSW optimization | Faster matched build, no unacceptable recall loss | **PASS** |
| Durability | No lost acknowledged writes in tested crash scenarios | **PASS** |
| Trust | Measured precision/recall, no defined scope bypass | **PASS** |
| Security | Adversarial memory tests without critical violations | **PASS** |
| Token budgeting | Measured token savings and evidence-retention tradeoffs | **PASS** |
| Concurrency | Race-free and correct at tested load | **CONDITIONAL PASS** — race-free and error-free in normal builds; write p99 degrades above ~8 writers |
| Scale | Bounded resources, honest max validated size | **PASS** — 100K |
| Integration | Existing public behavior and fallback preserved | **PASS** |

**CONDITIONAL PASS rationale (concurrency):** correctness and race-freedom hold
at every tested level, but the single-writer design means write tail latency
reaches ~0.75 s at 8 writers and ~1.3 s at 32 writers over long runs, and a
frequent full index rebuild can block readers for hundreds of ms. This is a
documented scalability limit, not a correctness failure.

## Tests executed

| Suite | Where | Result |
|---|---|---|
| Full learning package | `go test ./internal/learning/` | PASS (3× repeat) |
| Full learning package, race | `go test -race ./internal/learning/` | PASS (417 s) |
| HNSW unit/invariant | `go test ./internal/hnsw/` | PASS |
| HNSW build timing | `TestHNSWBuildTiming` 1K–100K | measured |
| HNSW recall | `BenchmarkANNvsExact` 10K/25K/100K | 1.000 / 1.000 / 0.9949 |
| Crash injection | `TestPhase2Crash*` | PASS |
| Trust/security | `TestPhase2Adversarial*`, `CrossScope`, `Hostile`, `Unapproved` | PASS |
| Ranking/token | `TestPhase2RankingAlternatives`, `TestPhase2TokenSelectionStrategies` | measured |
| Concurrency matrix + soak | `TestPhase2ConcurrencyStress` | PASS |
| MCP server | `go test ./internal/mcpserver/` | PASS |
| Dashboard memory | `go test ./internal/endpoint/dashboard/` | PASS |
| CLI memory commands | `go test ./cmd/ -run 'Memory\|Learning'` | PASS |
| Full CLI build | `go build ./...` | PASS |

## Commits (local only, on `test/beacon-v6-phase2-opencode`)

- `d15a8bca` phase2(hnsw): slice adjacency + cached edge distances + reusable search scratch
- `c6b27ade` phase2(durability): subprocess crash-injection harness and fault points
- `ee7c71ac` phase2(trust): adversarial matrix + overlap-ratio contradiction rule
- `2baa94f8` phase2(ranking/token): weightable RRF, larger labeled set, held-out eval
- `d7d6095b` phase2(concurrency+integration): stress harness, lifecycle tests, repro script
- `76109dc9` phase2(test): fix crash-child kill race; race-aware busy tolerance
- (reports) phase2: reports and executive results

## Honest limitations

- macOS only; no Linux/WSL2 execution or neural inference in this session.
- HNSW 100K baseline was extrapolated; 1M was out of scope and not run.
- HNSW build is not bit-reproducible (pre-existing; time-seeded RNG and arbitrary
  layer entry). Recall is stable.
- Contradiction recall is 0.758 (cross-kind misses) and dense-corpus cross-talk
  remains 37 pairs for human review.
- Power-loss durability is not simulated; only process crash.
- Token-budget selection loses ~6% of findings; not an unconditional win.
