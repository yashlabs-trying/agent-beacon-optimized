# Agent evaluation report

Status: **NOT MEASURED.**

## Why no results are reported

An end-to-end agent comparison needs a runnable agent loop with a language
model, a tool executor, and objective per-task checks. None is available in this
environment:

- No model endpoint or authorized inference environment; the default path must
  not require a hosted service.
- No agent runtime / SWE-bench-compatible harness installed.
- The built `beacon` binary cannot run on this machine (application-control
  policy blocks locally compiled binaries); all work was validated in WSL.

Reporting agent-task success, token, or cost numbers would be fabrication, so
none are given.

## What the retrieval-only evidence does and does not show

- It shows retrieval quality improved on a labeled set (recall@5 0.41 → 0.71) and
  latency improved by ~300× on scan-bound lexical queries.
- It does **not** show that agent task success improves, nor that end-to-end
  latency or cost improves. An accelerated query does not imply a better agent.

## Planned comparison (not run)

Arms A–E (no memory / original Beacon / v2 / v3 / best V4), identical model,
prompts, tools, budgets, verification. Task families: repository debugging,
reusing prior fixes, applying conventions, avoiding failed approaches, handling
superseded knowledge, cross-harness reuse. Per task: success, harmful
retrievals, tokens, tool calls, latency, cost, tasks-per-dollar, recovery.
Replicates for confidence intervals.

## Unblock

A machine with a model endpoint and the ability to run the built binary, or a
harness that embeds the `learning` package directly with a local model.
