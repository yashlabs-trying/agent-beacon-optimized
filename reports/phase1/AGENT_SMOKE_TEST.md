# Phase 1 — Real Coding-Agent Smoke Test (22 October 2026 Mac)

The Apple M5 Mac ran a local Ollama `qwen2.5-coder:7b` model (4.7 GB quantized download) and a three-task disposable Python coding-agent harness. The model, tool schema, temperature=0, seed=42, context/prediction limits, and test fixtures were identical across no-memory, original Beacon, and V6 arms. Memory arms seeded the same synthetic project-scoped notes, then queried **actual original/V6 `beacon memory list --json` CLIs** from isolated workspaces. The agent could read fixture files, edit only the task source, and run objective tests. No external network or real secrets were in the tasks. Reproduce with `python3 reports/phase1/agent_harness.py --run-root NEW_EMPTY_DIR --original PATH_TO_BASELINE_BEACON --v6 PATH_TO_V6_BEACON --seed PATH_TO_PHASE1SEED --model qwen2.5-coder:7b --output RESULTS.json` while local Ollama runs.

Ollama's first 1.5B and 7B attempts emitted JSON tool requests in text rather than native API `tool_calls`; the unadapted harness recorded zero calls and 0/3 successes in every arm. Those traces are retained in `artifacts/agent_1p5b.json` and `artifacts/agent_7b.json`, **not** counted as agent outcomes. The harness now accepts an exact JSON tool-call object in message content as an adapter and records `call_sources`; it still executes the real requested tools with a 12-call ceiling.

| Task | No memory | Original Beacon | V6 Beacon |
|---|---|---|---|
| Feature flag boolean parsing | Fail, 8 tool calls | Pass, 8 | Pass, 8 |
| Retry delay convention | Pass, 8 | Pass, 8 | Pass, 8 |
| Slug supersession convention | Fail, 7 | Fail, 8 | Fail, 8 |
| **Total** | **1/3** | **2/3** | **2/3** |

Original and V6 retrieved identical note IDs per task: `feature-distractor,feature-current`, `retry-distractor,retry-current`, and `slug-distractor,slug-current`; the superseded `slug-old` was absent. Retrieval took 9.84–12.22 ms except one cold V6 run in the unadapted trace (903.86 ms); adapted per-agent elapsed time was about 11.5–15.3 seconds. Prompt tokens processed ranged 4,943–7,140; generated tokens 224–369. Per-run success, retrieval IDs/latency, token counts, source SHA, final text, and tool trace are in `artifacts/agent_7b_adapted.json`.

This demonstrates that retrieved notes can help this local model on one fixture, but does **not** demonstrate a V6 task-success advantage over original Beacon: the CLI results and both memory arms were effectively identical. The slug failure shows model/tool-loop limits despite relevant notes. Three tiny synthetic tasks and one deterministic run per arm are insufficient for a broad agent-quality claim. The earlier Mac access blocker below was resolved for this revalidation.

## Prior Phase 1 report (preserved below)

# Phase 1 — Real Coding-Agent Smoke Test

## Status: **NOT TESTED**

## Blocker

The agent smoke test requires a local coding model and a harness. In Phase 1:

- The **Mac** (the designated machine for local model inference) is unreachable:
  `Permission denied (publickey,password,keyboard-interactive)` on
  `<user>@192.0.2.10`.
- No coding model or agent harness is installed on the WSL2 laptop, and the brief
  forbids new paid compute (the GPU pod is a paid rental and was not used for
  Phase 1).

No agent task was executed, so no success/token/cost numbers are reported.
Substituting simulated outcomes is prohibited.

## Prepared (for Phase 2, once a machine/model is available)

- Arm A: no memory. Arm B: original Beacon. Arm C: V6.
- 2–3 executable tasks with objective checks: reuse a prior debugging memory,
  apply a repository convention from memory, ignore a superseded memory.
- Identical model, tools, prompt budget, execution limits; workspace reset
  between runs.
- Record: success, tool calls, tokens, retrieval latency, end-to-end latency,
  correct/harmful memory use.

## Unblock

Authorize the Mac SSH key (add `~/.ssh/id_ed25519.pub` to the Mac's
`~/.ssh/authorized_keys`), or run the harness on a machine with a local model.
Until then this gate is **NOT TESTED**.
