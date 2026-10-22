# Phase 1 — Real Coding-Agent Smoke Test

## Status: **NOT TESTED**

## Blocker

The agent smoke test requires a local coding model and a harness. In Phase 1:

- The **Mac** (the designated machine for local model inference) is unreachable:
  `Permission denied (publickey,password,keyboard-interactive)` on
  `user@192.168.1.10`.
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
