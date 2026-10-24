# Phase 4A — Evidence Audit (Phases 1–3D)

A consolidated, non-inflated matrix of what the existing workstreams actually
established, separated by evidence class. Sources: `reports/phase1/*`,
`reports/phase2/*`, `reports/phase3/*`, `reports/V6_CONSOLIDATED_RESULTS.md`,
`reports/HARDWARE_EXECUTION_REPORT.md`, and the committed raw evidence.

Hardware/corpus/workload/units are preserved where a number is quoted. Where the
original report gave only a summary, the summary is quoted and marked as such.

## 1. Evidence classes

- **INDEPENDENT** — measured against a frozen baseline or an independent
  implementation, on identical fixture/config.
- **DEV-FIXTURE** — measured only on development fixtures (not held-out).
- **FROZEN-REPRO** — reproduced against frozen/locked fixtures.
- **SCRIPTED** — deterministic scripted harness validation, not a model.
- **REAL-MODEL** — a real model produced the outcome.
- **BLOCKED/FAILED** — not run, or ran and failed the gate.
- **UNSUPPORTED** — a claim that no evidence yet supports.

## 2. Retrieval quality

| Claim | Value | Class | Source |
|---|---|---|---|
| Lexical R@5 (17-case, Phase 1) | 0.412 | DEV-FIXTURE | `V6_CONSOLIDATED_RESULTS.md` |
| Hashed-semantic R@5 (17-case) | 0.706 | DEV-FIXTURE | `V6_CONSOLIDATED_RESULTS.md` |
| Neural BGE R@5 (17-case) | 1.000 | REAL-MODEL (dev) | `NEURAL_SMALL_SCALE.md` |
| Locked lexical R@5 (48 q) | 0.167 | FROZEN-REPRO | Phase 3A + Phase 4A |
| Locked hashed R@5 (48 q) | 0.875 | FROZEN-REPRO | Phase 3A + Phase 4A |
| Locked hybrid RRF/ANN R@5 | 0.875 | FROZEN-REPRO | Phase 3A |
| Locked real-neural R@5 | **0.979** | REAL-MODEL | **Phase 4A (new)** |
| ANN recall@10 (100K, M=64) | 0.9953 | FROZEN-REPRO | `ANN_SMALL_SCALE.md`, Phase 3A |
| ANN recall@10 (1M, M=16) | 0.4660 | DEV-FIXTURE | `V6_CONSOLIDATED_RESULTS.md` |

Notes: the 17-case and 48-query sets are **different relevance definitions and sizes**;
scores must not be compared across them. The 0.412→0.706→1.000 progression is on the
17-case development set; the 0.167→0.875→0.979 progression is on the locked 48-query set.

## 3. Latency / cost

| Claim | Value | Class | Source |
|---|---|---|---|
| Lexical 1M old-only p50 | 1.60 ms (laptop) / 2.87 ms (pod) | INDEPENDENT | `V6_CONSOLIDATED_RESULTS.md` |
| Lexical 1M no-match p50 | 0.61 ms / 1.69 ms | INDEPENDENT | same |
| ANN search p50 (100K) | 1.20 ms | DEV-FIXTURE | same |
| Exact vector p50 (100K) | 38 ms (synthetic) | DEV-FIXTURE | same |
| ANN build (100K, M=64) | 506 s (pod) / 102 s (WSL bounded) | DEV-FIXTURE | same + Phase 3A |
| Neural p50/p95 (locked, over LAN) | 43.6 ms / 133 ms | REAL-MODEL | **Phase 4A (new)** |
| 8-client write p99 | 118–135 ms | FROZEN-REPRO | Phase 3C |
| 16-client write p99 | 744–795 ms (**FAIL ≥250 ms**) | FROZEN-REPRO | Phase 3C |
| Context tokens (17 q) | 1,242 | DEV-FIXTURE | `V6_CONSOLIDATED_RESULTS.md` |

## 4. Correctness / reliability / security

| Claim | Value | Class | Source |
|---|---|---|---|
| Full CLI suite | PASS | SCRIPTED/GO | Phase 3A–3D; Phase 4A |
| `go test -race` (learning+hnsw) | PASS | GO | Phase 3B/3C; Phase 4A (subset) |
| Crash durability (7 fault points) | PASS | GO | Phase 3A/Phase 4A |
| Concurrency ≤8 clients | PASS | GO | Phase 3A–3C |
| Cross-scope leakage | 0 | GO | Phase 2/Phase 4A |
| Hostile memory content | PASS | GO | Phase 2/Phase 4A |
| Locked contradiction precision | 1.000 | FROZEN-REPRO | Phase 3A/3B/4A |
| Locked contradiction recall (default) | **0.500** | FROZEN-REPRO | Phase 3B/4A |
| Cross-kind contradiction recall (opt-in) | 1.000 | DEV-FIXTURE | Phase 3B |
| Token budgeting evidence loss | loses 3/109 | DEV-FIXTURE | Phase 3A/3B |

## 5. Agent value

| Claim | Value | Class | Source |
|---|---|---|---|
| Phase 1 3-task Mac pilot | none 1/3, original 2/3, V6 2/3 | REAL-MODEL (dev) | `PAIRED_AGENT_EVALUATION.md` §5 |
| Phase 3D 6-task scripted reference | 6/6 all arms | SCRIPTED | Phase 3D |
| Phase 3D 6-task scripted noop | 0/6 all arms | SCRIPTED | Phase 3D |
| Phase 3D 6-task real-model | BLOCKED | BLOCKED | Phase 3D |
| **Phase 4A 6-task real-model** | **none 4/6, original 6/6, V6 6/6** | **REAL-MODEL** | **Phase 4A (new)** |

## 6. Known limitations carried forward (must remain opt-in / unresolved)

- Phase 3A neural retrieval did not run **at the time**; **Phase 4A ran it** (real).
- Phase 3D six-task model evaluation was **blocked**; **Phase 4A ran it** (real).
- Scripted 6/6 outcomes do **not** demonstrate model capability (still true — Phase 4A
  keeps them labelled validation).
- 16-client write p99 remains **above target** (744–795 ms).
- Default contradiction recall remained **limited** (0.500 locked).
- Cross-kind contradiction, ANN, neural, hybrid, and token budgeting remain **opt-in**.
- V6 vs original Beacon **agent advantage is not demonstrated** (6/6 = 6/6).

## 7. Claims NOT yet supported by evidence

- "V6 improves agent task success vs original Beacon" — **UNSUPPORTED** (6/6 = 6/6).
- "V6 retrieval is production-ready" — **UNSUPPORTED** (neural needs a sidecar; ANN
  build cost; hashed matched by FTS5).
- "Token budgeting saves tokens without loss" — **UNSUPPORTED** (loses findings).
- "Contradiction detection is reliable" — **UNSUPPORTED** at default (recall 0.50).
- "ANN scales to 1M at M=64 in reasonable time" — **UNSUPPORTED** (≈83 min build).
- "16-client concurrency is supported" — **UNSUPPORTED** (p99 > target).

## 8. Integrity statements

- No locked fixture, expected outcome, or frozen reference was edited.
- The `agent_eval.py` change (Phase 4A) adds an optional `--ollama-url` and a more
  tolerant JSON-in-content parser; it does not change the scorer, the tests, the prompt,
  or any locked label. It is documented in `HARNESS_VALIDATION.md` and `SIX_TASK_PILOT.md`.
- Scripted validation results are never reported as model outcomes.
