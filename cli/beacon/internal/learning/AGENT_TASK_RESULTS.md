# Agent task results

Status: **NOT MEASURED.**

## Environmental blocker

No executable agent environment is available:

- No local inference runtime and no authorized inference endpoint.
- No coding-agent CLI or SWE-bench-compatible harness installed.
- The built `beacon` binary cannot run on this machine: Windows Smart App
  Control blocks locally compiled binaries, so even the existing CLI/MCP path
  could not be exercised end to end (all work was validated inside WSL).

Per the rules, no simulated or fabricated agent success numbers are reported.

## What is available instead

- A runnable retrieval harness (`TestVerifyHarness`, `TestSemanticRetrievalQuality`,
  `TestANNRecallSynthetic`, `TestContextBudgeting`) that measures the retrieval
  layer the agent would call.
- The measurements in `V5_CONSOLIDATED_BENCHMARKS.md` are the honest proxy.

## Planned harness (not run)

- Arms A–E: no memory / original Beacon / V2 / V4 / V5.
- Identical model, prompt, tools, budgets, verification; isolated workspaces
  reset between runs; repeated runs for variance.
- Task families: fix a prior regression, reuse a prior debugging solution,
  dependency conflict, project convention, avoid a failed approach, recognize a
  superseded instruction, cross-harness reuse, resist misleading memory.
- Per task: success (objective), relevant/harmful memory retrieved, tokens,
  context tokens, latency, tool calls, cost, failure attribution.
- Minimum 10 distinct tasks, ideally 20–30 with replicates.

## Unblock

A machine with a model endpoint (local or authorized) and the ability to execute
the built binary, or an agent loop that embeds the `learning` package directly.
