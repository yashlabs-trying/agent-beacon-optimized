# Phase 4A — Benchmark D: Six-Task Paired Agent Pilot

**Gate: PASS (executed).** The model-dependent three-arm run that Phase 3D could not
execute now ran with a real model. **V6 did not beat original Beacon** — both scored
6/6 with identical retrieved evidence.

- Fixture: `reports/phase3/heldout/agent_tasks_v1.json`, SHA-256 `409195B0…C530` (6 tasks).
- Model: `qwen2.5-coder:7b` via Ollama on the Mac M5 GPU.
- Arms: A `none`, B `original` (`99c70908`), C `v6` (`bcf3a1fd`).
- Raw evidence: `RAW_EVIDENCE/six_task_pilot.{json,txt}`,
  `RAW_EVIDENCE/validation_reference.json`, `RAW_EVIDENCE/validation_noop.json`.

## 1. Setup

Same model, system prompt, task request, tool schema, limits (8 turns / 12 tool calls),
timeouts, and evaluator for all three arms. Each (task, arm) runs in a fresh directory;
objective tests live outside the agent-readable workspace; both memory arms seed
`<task>-current` + `<task>-distractor` and retrieve with the same query
`beacon memory list --json --limit 5 --query "<task words>"`.

## 2. Results

| Arm | Task success | Avg agent s | Prompt tok | Gen tok | Avg retrieval ms |
|---|---:|---:|---:|---:|---:|
| A `none` | **4/6** | 13.03 | 29,123 | 1,468 | 0.00 |
| B `original` | **6/6** | 14.63 | 28,728 | 1,730 | 13.67 |
| C `v6` | **6/6** | 15.01 | 28,733 | 1,730 | 13.75 |

Per task:

| Task | none | original | v6 | retrieved IDs (memory arms) |
|---|---|---|---|---|
| retry_after | Pass | Pass | Pass | `retry_after-distractor, retry_after-current` |
| stable_cursor | Pass | Pass | Pass | `stable_cursor-distractor, stable_cursor-current` |
| event_dedup | **Fail** | Pass | Pass | `event_dedup-distractor, event_dedup-current` |
| utc_conversion | **Fail** | Pass | Pass | `utc_conversion-distractor, utc_conversion-current` |
| lease_fence | Pass | Pass | Pass | `lease_fence-distractor, lease_fence-current` |
| cache_key | Pass | Pass | Pass | `cache_key-distractor, cache_key-current` |

## 3. Interpretation

- **Memory helps vs no memory**: 6/6 vs 4/6. Both `none` failures (`event_dedup`,
  `utc_conversion`) are *caused by absent memory* — without the project convention the
  model implemented the wrong semantics. This is the intended positive signal, but n=6.
- **V6 does not beat original Beacon**: both 6/6. The two memory arms retrieved **byte-
  identical IDs** (`<task>-distractor, <task>-current`), so retrieval identity was
  constant and only the CLI differed. A V6 advantage cannot appear when the retrieved
  evidence is the same. **No V6-over-original claim is made.**
- **Latency/tokens**: memory adds ≈14 ms retrieval and ≈0 net prompt tokens (the notes
  displace nothing meaningful); agent wall time rose slightly (13.0→14.6→15.0 s) but this
  is within per-call model variance, not a retrieval effect.
- **Call format**: the model emitted JSON-in-content (no native tool calls); all tool
  calls validated. This is a model property, not a harness or memory property.

## 4. Honesty notes

- One trial per (task, arm) — the local setup does not support three seeds without
  unacceptable cost. Six tasks cannot support a statistical claim; results are descriptive.
- The scripted reference/noop runs (6/6 and 0/6) remain **validation only** and are not
  model outcomes.
- This pilot used the *distractor+current* pair from the fixture, which the model mostly
  ignored; the tasks are solvable without memory, so the observed lift is small. The
  expanded protocol (§`EXPANDED_EVALUATION_PROTOCOL.md`) uses tasks that genuinely
  require the convention.

## 5. Comparison to prior context

Phase 1's 3-task Mac pilot (different fixture) reported none 1/3, original 2/3, V6 2/3 —
also no V6 advantage. Phase 4A's 6-task pilot reproduces that pattern at larger n:
**memory helps, V6 does not separate from original.**

## Conclusion

**Benchmark D: PASS (executed).** The harness runs end-to-end with a real model and
distinguishes memory vs no-memory; it provides **no evidence** that V6 improves agent
outcomes over original Beacon. A ≥30-task, multi-seed, memory-required study is the
correct next step.
