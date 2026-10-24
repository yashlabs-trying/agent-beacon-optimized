# Phase 3D — Paired Agent Evaluation (Evaluation B)

**Status: harness built and validated; the model-dependent three-arm run is BLOCKED
(no protocol-compatible local model). No agent task-success outcome is claimed.**

## 1. Why the model run is blocked

The prescribed protocol uses a local Ollama coding model
(`qwen2.5-coder:7b`, or `1.5b` in the Phase 1 harness). In this environment:

| Prerequisite | Result |
|---|---|
| Ollama (`ollama` / `ollama.exe`) | not installed (WSL or Windows) |
| `llama.cpp` / `vLLM` / LM Studio | not installed |
| Listening LLM ports (11434, 8000, 1234, …) | none |
| Cached chat/coder model weights | none matching the protocol |
| Apple M5 (Ollama host) | unreachable |

Cached models are `Qwen2.5-0.5B` (base) and `SmolLM2-135M`; the other two cache entries
(`opt-125m`, `gpt-oss-20b`) have no weights. A direct probe of `Qwen2.5-0.5B` on CPU
(`torch 2.14.0+cpu`, `transformers 5.17.0`) produced degenerate, non-tool-call output, so
it is unusable for this agent loop. Downloading a model is not permitted by the brief.

**Exact missing prerequisite:** a locally served instruction/coder chat model compatible
with the harness tool-calling loop (e.g. Ollama `qwen2.5-coder:7b`), or a reachable Mac
running it.

## 2. Harness built (harness-only change, justified)

- `reports/phase3/part4/harness/agent_eval.py` — six-task three-arm harness with a
  pluggable backend, tests kept outside the agent workspace, access-controlled tools,
  and paired scoring.
- `cli/beacon/internal/learning/testdata/phase3dseed/main.go` — seeds each task's
  `approved_note` and `distractor_note` from the locked fixture (testdata only; excluded
  from shipping builds).
- Binaries built for the arms: `beacon-original` (99c70908) and `beacon-v6` (d12e413a).

## 3. Harness validation (NOT model outcomes)

Run with the deterministic scripted backend to prove the protocol, isolation, memory
seeding/retrieval, and scoring — before any model outcome.

### `--script reference` (agent applies the known-correct solution)

| Task | none | original | v6 | retrieved IDs (memory arms) |
|---|---|---|---|---|
| retry_after | Pass | Pass | Pass | `retry_after-distractor, retry_after-current` |
| stable_cursor | Pass | Pass | Pass | `stable_cursor-distractor, stable_cursor-current` |
| event_dedup | Pass | Pass | Pass | `event_dedup-distractor, event_dedup-current` |
| utc_conversion | Pass | Pass | Pass | `utc_conversion-distractor, utc_conversion-current` |
| lease_fence | Pass | Pass | Pass | `lease_fence-distractor, lease_fence-current` |
| cache_key | Pass | Pass | Pass | `cache_key-distractor, cache_key-current` |
| **Total** | **6/6** | **6/6** | **6/6** | |

### `--script noop` (agent makes no change)

| Task | none | original | v6 |
|---|---|---|---|
| all six | Fail | Fail | Fail |
| **Total** | **0/6** | **0/6** | **0/6** |

This confirms: the six tasks load; the reference solutions pass the objective tests; the
noop agent fails them; tests live outside the workspace; and both the original and V6
CLIs seed and retrieve the same task notes. **These are validation runs, not agent
outcomes.**

## 4. Model-dependent paired outcomes

**NOT RUN (BLOCKED).** No no-memory/original/V6 task-success numbers exist. Per the
brief, no outcome is fabricated, and no V6-versus-original advantage is claimed.

| Comparison | Result |
|---|---|
| No memory → original Beacon (6 tasks) | **NOT RUN** |
| Original Beacon → V6 (6 tasks) | **NOT RUN** |

## 5. Prior pilot (context only, different fixture)

The Phase 1 three-task smoke test on the Mac (`qwen2.5-coder:7b`) reported
no-memory 1/3, original 2/3, V6 2/3, with both memory arms retrieving identical IDs —
i.e. **no V6 advantage**, and the `slug` task failing in every arm. That used a
different, smaller task set and is retained only as prior context, not as a Phase 3D
result.

## 6. Limitations

- One trial per condition per task is planned (the brief permits a single trial when the
  local setup cannot support repeats).
- Six tasks cannot support statistical claims; results would be descriptive.
- A successful task could not be attributed to better retrieval without retrieval-ID and
  trace evidence (which the harness records).

## 7. Recommendation

Provision the protocol model locally (Ollama `qwen2.5-coder:7b`) or a reachable Mac, then
run the unchanged harness. The Phase 3 plan's own guidance is that the six-task set
validates the harness; a superiority claim would require ≥30 locked tasks and a
three-seed comparison.
