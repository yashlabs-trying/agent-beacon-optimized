# Beacon V7 — Final Engineering Verdict (hardening continuation)

## 1. What this continuation did

Recovered the Codex work from Phase A (`e1a2ada6`), preserved the uncommitted Phase B
work (a regression test), completed Phase B, made a Phase C performance pass, and
measured Phases D/H. Phases E–J are reported honestly with explicit gaps.

## 2. Provenance (verified)

| Ref | Commit | Role |
|---|---|---|
| Original | `99c709081ad49da1c5ade205929b61b19360ed9b` | frozen baseline |
| V6 integration | `209a987bc96c29b5e5d811a794d0357f20214463` | frozen |
| Phase 5 | `02e34f98dec1c5d34a2f0d52904969edc39e389f` | frozen eval |
| V7 dev | `7b605aa84f0f570ba265fbda17fe6d99e927b10e` | V7 parent |
| Phase A | `e1a2ada6531cc9f5c4748050c0d5fe33e7ffa0cb` | audit |
| Phase B | `e240876658da0ebd5edce6ef1b168dafe3b856f6` | attribute fix |
| Phase C | `2a62fd4fbe7e2bf1e69061766cda743db1d345aa` | perf |

Original, V6, Phase 5, and V7-dev refs were not modified.

## 3. Measured results

**Phase B (correctness).** "What is the environment for the orion service?" previously
selected `[owner environment]`; now selects `[environment]`. Root cause: the requested
attribute slot was the *last* content word. Fixed to the first content noun + verb
noise. 12/12 question tests pass; full suite ok 58.0 s.

**Phase C (performance, warm, 1,000-memory store).**

| Metric | Before | After |
|---|---:|---:|
| `SearchQuestion` B/op | 248,382 | **207,988** (−16%) |
| `SearchQuestion` allocs/op | 2,099 | **2,007** (−4%) |
| `ListMemories` (Limit 8) | ~58 µs, 64 allocs | unchanged |

pprof: warm CPU is SQLite-bound; allocations were `SelectTrustContext`-bound.
Cold first-use FTS penalty (original 14.7 ms vs V7 43.7 ms, Phase A) **remains open**.

**Phase D (concurrency).** 8 clients: reads p99 11.9 ms, writes p99 130 ms, 0
errors/busy/stale. 16 clients: write p99 **835–935 ms** — **FAIL** vs 250 ms. 32/64/128
clients and 1M records: **NOT TESTED**.

**Phase H (tests).** Learning suite ok 58.0 s; race subset ok 39.3 s; vet/build/gofmt
clean. Full repo CLI suite, ANN/neural harnesses, MCP integration, cross-platform:
**NOT TESTED**.

## 4. Honest limitations

- Authenticated provenance is **not implemented** (`source:verified` is an unauthenticated
  string).
- Feedback-driven learning is **not implemented**.
- Self-healing is rebuild-recovery, not active repair.
- Cold retrieval penalty open; 16-client writer tail open.
- Fresh model-backed ablations and the final 500-task holdout were **not run** (by design).

## 5. Release gate verdict

**NOT RELEASE-READY.** Blocking: gate 5 (16-client writer p99 FAIL); conditional:
gate 2 (provenance) and gate 4 (cold performance); partial: gate 7 (test coverage).

## 6. Five highest-impact next steps

1. Eliminate the cold first-use FTS penalty (prepare derived state off the query path).
2. Authenticated provenance (Ed25519 ingestion identities + signatures).
3. Durability-safe write-concurrency strategy for the 16-client tail (or formally accept
   the ≤8-client budget in the release criteria).
4. Run a fresh, matched, model-backed ablation of V7 configurations on a held-out suite
   (Phase I) — the real evidence gate.
5. Implement feedback-driven learning if the "self-improvement" objective is to be
   claimed; otherwise remove the claim.

## 7. What this does and does not establish

- **Does:** V7 fixes a real question-understanding defect; warm allocations are lower;
  durability/race/vet pass; V7 inherits the V6 trust/sufficiency behavior.
- **Does not:** prove V7 beats original on agent tasks; fix cold retrieval or the
  16-client writer tail; provide authenticated provenance or self-improvement.
