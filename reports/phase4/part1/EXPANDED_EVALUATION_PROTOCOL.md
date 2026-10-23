# Phase 4A — Benchmark E: Expanded Evaluation Protocol (≥30 Tasks)

**Gate: PASS (protocol designed and frozen).** No model experiment is run here; this is
the preregistered protocol that Phase 4B must execute. It is written **before** any
held-out outcome is observed.

## 1. Objective

Answer the central question with statistical honesty: do agents using **V6** complete
more real tasks, correctly and efficiently, than agents using **original Beacon** or
**no memory**?

## 2. Arms

| Arm | Memory | Retrieval |
|---|---|---|
| A | none | — |
| B | original Beacon `99c70908` | lexical |
| C | V6 `bcf3a1fd` | lexical + hashed (default opt-in off) |
| D (added) | V6 + neural | BGE-small via sidecar |

Arm D isolates whether the *only* demonstrated retrieval win (neural) converts to task
success. It is the highest-value comparison and is new relative to Phase 3D.

## 3. Task design (≥30 tasks)

Ten categories, ≥3 tasks each, each with a reproducible starting environment, a
memory-required convention, distractor memories, and an objective automated check
(hidden unit tests kept outside the agent workspace):

1. Reuse a successful debugging fix (prior trace → correct patch).
2. Apply a project-specific convention (e.g. UTC handling, retry semantics).
3. Learn from a failed previous attempt (avoid the known-bad approach).
4. Avoid repeating a mistake flagged by a gotcha memory.
5. Honour an API contract (field names, error shapes).
6. Update stale knowledge (supersede old convention with new).
7. Resolve contradictory memories (must pick the current one).
8. Distinguish projects/versions (do not apply project A's rule to project B).
9. Recover context across sessions (only memory carries the key).
10. Identify when retrieved memory should be ignored (memory-neutral decoys).

Each task must be **memory-required**: without the correct memory, a competent model
reliably produces a different, failing result. This is enforced by construction and
verified by the no-memory arm (task must fail ≥2/3 seeds without memory).

## 4. Splits, freezing, and hashing

- **Development split (15 tasks):** prompt/retrieval tuning allowed.
- **Held-out split (≥15 tasks):** frozen before model runs; hashed.
- Fixture format: `{task_id, category, source, request, memory_history[], gold_tests,
  distractor_memories[]}`.
- Freeze procedure: `sha256` (CRLF→LF) of each fixture + a `LOCK_MANIFEST.json` with a new
  `dataset_id`, exactly as Phase 3 did. Run `validate_lock.py` before and after.
- **Rule:** no prompt, retrieval rule, or scorer threshold may be changed after observing
  held-out outcomes; doing so invalidates the independent test.

## 5. Trials and order

- 3 seeds per (task, arm); seeds are **dependent** observations of the same task.
- Counterbalance arm order per task (rotate A→B→C→D) to remove warm-cache/order bias.
- Cold vs warm retrieval recorded separately.
- Total executions: 4 arms × 30 tasks × 3 seeds = 360; run in bounded batches, abort if
  hardware/cost limits are hit (the brief forbids automatic 270–360 runs when constrained).

## 6. Measurements per (task, arm, seed)

Functional success (primary), correctness of final artifact, relevant-memory usage
(retrieved IDs + whether the used fact came from memory), failures caused by incorrect
memory, tool-call/execution errors, wall latency, retrieval latency, prompt/output
tokens, and cost per correctly-completed task where measurable.

## 7. Statistics

- Primary: paired task-success difference between arms, per task (paired by task, seeds
  aggregated within task as dependent observations).
- Use a paired method appropriate to binary outcomes (e.g. exact McNemar / sign test for
  B vs C and A vs C), reporting exact denominators and per-task outcomes.
- Report improvement and regression counts separately, not just net.
- Report uncertainty intervals; **never** claim superiority from six tasks.
- Pre-register the primary comparison (C vs B) and the secondary (D vs B, B vs A).

## 8. Scoring independence

Reuse the Phase 4A harness guarantees: functional success only, retrieval-identity
independent, citation not rewarded, tests outside the workspace, negative controls in CI.

## 9. Stop / promote criteria

- Promote neural to default **only if** arm D beats B on the held-out split with a paired
  test whose interval excludes 0, at acceptable latency/token cost.
- Keep hashed/ANN/token-budgeting/cross-kind contradiction opt-in unless their own
  arm-level evidence justifies promotion.
- If no arm separates, **stop optimizing components** and focus on ranking/selection.

## 10. Reproduction

`reports/phase4/part1/run_phase4a.sh` builds the arm binaries and runs the six-task
pilot; the ≥30-task run reuses `reports/phase3/part4/harness/agent_eval.py` with
`--fixture <locked ≥30-task fixture>` and `--ollama-url <LAN host>`.

## Conclusion

**Benchmark E: PASS (protocol).** A frozen, held-out, multi-seed, memory-required protocol
is specified. Its execution is the single highest-priority next experiment.
