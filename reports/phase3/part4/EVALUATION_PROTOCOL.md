# Phase 3D — Evaluation Protocol

## 1. Fixture and provenance

- Locked fixture: `reports/phase3/heldout/agent_tasks_v1.json`
  (`dataset_id: beacon-v6-phase3-agent-tasks-v1`), SHA-256
  `409195B01ED6E2DB55BCDF4EF5D8710DEEEDFBFFE5320EB4D5722215AB0DC530`.
- Six tasks: `retry_after`, `stable_cursor`, `event_dedup`, `utc_conversion`,
  `lease_fence`, `cache_key`. Each has `source`, `request`, `approved_note`,
  `distractor_note`, and objective `tests`.
- Provenance: Phase 3 prep `c9275d84`; the fixture is unchanged and was not edited.

## 2. Arms

| Arm | Implementation |
|---|---|
| A `none` | no memory (no seed, no retrieval) |
| B `original` | original Agent Beacon `99c709081ad49da1c5ade205929b61b19360ed9b` |
| C `v6` | verified V6 `d12e413a08a20d1f2abbe80a91daa1386e5edfd6` (includes Phase 3C pooling) |

The arms differ **only** in the memory capability. Same model, system prompt, task
request, tool schema, tool limits, timeouts, and evaluator for all three.

## 3. Isolation and blindness

For every (task, arm) a fresh directory is created:
- `workspace/` — agent-readable; contains **only** the task source file.
- `evaluator/` — holds the objective `test_solution.py`; **not** agent-readable.
- `memory.db` — the arm's memory store (memory arms only).
- `logs/` — runtime log path for the CLI.

The agent tools are access-controlled: `read_file` and `write_file` accept only the task
source filename; the tests are never placed in the workspace. `run_tests` copies the
agent's current source into the evaluator directory and runs the hidden tests there,
returning only pass/fail + unittest output.

## 4. Memory seeding and retrieval (memory arms)

- Seed: `phase3dseed <task> <workspace> <db> <fixture>` inserts
  `<task>-current` (the `approved_note`) and `<task>-distractor` (the `distractor_note`)
  as approved project-scoped convention memories.
- Retrieval: the arm's real CLI,
  `beacon memory list --json --limit 5 --query "<task words>" --project <workspace> --log-path <logs>`.
  The same query is used for both memory arms; retrieved IDs and latency are recorded.

## 5. Agent loop

- System prompt: coding-agent instructions; retrieved notes are reference data.
- User prompt: the task `request` + retrieved notes (or `(none)`).
- Up to 8 model turns and at most 12 tool calls.
- Tool-call parsing accepts native `tool_calls` and the JSON-in-content fallback used by
  the Phase 1 harness (`call_sources` recorded).

## 6. Scoring

- Objective `unittest` run from the evaluator directory after copying the agent's final
  source in. `success = (exit code 0)`.
- The rubric is the fixture's own tests; it was **not** modified.
- Scoring is blind to the expected answer at task time (tests live outside the
  workspace); the harness never reveals test source to the agent.

## 7. Backends

- `--backend ollama`: the prescribed real local model (Ollama `/api/chat`,
  `temperature=0, seed=42, num_ctx=4096, num_predict=512`). **Unavailable in this
  environment → the model run is BLOCKED.**
- `--backend scripted --script reference|noop`: deterministic agent used **only** to
  validate the harness and scoring. These are **not** model outcomes and are labelled as
  validation.

## 8. Protocol source and deviations

- Derived from the existing `reports/phase1/agent_harness.py` and
  `reports/phase1/AGENT_SMOKE_TEST.md`, extended to the locked six tasks per the Phase 3
  plan (step 2: "make the evaluator load the six locked tasks while keeping objective
  tests outside the agent-readable workspace").
- Deviation 1: the prescribed local model (`qwen2.5-coder:7b` via Ollama) is unavailable;
  the model-dependent three-arm run is BLOCKED.
- Deviation 2: only one trial per condition per task was planned (the brief permits this
  when the local setup cannot support repeats without unacceptable cost).

## 9. Reproduce

```
cd cli/beacon
go build -o <bin>/beacon-v6 .
# original (baseline worktree at 99c70908)
( cd <baseline>/cli/beacon && go build -o <bin>/beacon-original . )
go build -o <bin>/phase3dseed ./internal/learning/testdata/phase3dseed

python3 reports/phase3/part4/harness/agent_eval.py \
  --run-root <empty-dir> \
  --fixture reports/phase3/heldout/agent_tasks_v1.json \
  --original <bin>/beacon-original --v6 <bin>/beacon-v6 --seed <bin>/phase3dseed \
  --backend ollama --model qwen2.5-coder:7b \
  --output results.json
```

Raw evidence: `RAW_EVIDENCE/validation_reference.{txt,json}`,
`RAW_EVIDENCE/validation_noop.{txt,json}`, `RAW_EVIDENCE/bin/`.
