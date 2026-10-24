# End-to-end agent task evaluation (Phase 5)

Status: **NOT IMPLEMENTED**.

## Why

End-to-end agent evaluation requires a runnable agent loop with a language model,
a tool executor, a task harness and objective per-task checks. None is available
in this environment:

- No model endpoint or API key is available, and the default path must not
  depend on a hosted service (a project constraint).
- No agent runtime or tool sandbox is installed here.
- The repository's own MCP/CLI harness needs the built `beacon` binary to run
  end to end, and this machine's application-control policy blocks locally
  compiled binaries (all work in this branch was validated inside WSL).

Fabricating agent-task numbers would violate the project rules, so **no
agent-task results are reported**.

## Planned design (not run)

Arms, identical model/prompts/tasks/tools/evaluation rules:

- A: agent without memory
- B: agent with original Beacon memory
- C: agent with v2 lexical engine
- D: agent with the full v3 engine

Task families: repository debugging, reusing a prior fix, applying a project
convention, avoiding a previously failed approach, handling superseded
knowledge, cross-harness knowledge reuse.

Per task capture: objective success/failure; relevant memory retrieved;
irrelevant or harmful memory retrieved; total input/output tokens;
retrieval-context tokens; latency; tool-call count; estimated cost; recovery
from incorrect memory. Replicates: enough independent tasks per family to report
confidence intervals.

Decisive outcome: whether task completion improves without an unacceptable cost
increase.

## What would unblock it

A machine with a model endpoint (local or hosted) and the ability to run the
built `beacon` binary, or an agent harness that can embed the `learning` package
directly. The retrieval metrics in `SEMANTIC_BENCHMARKS.md` and
`TRUST_BENCHMARKS.md` are the available proxy evidence until then.
