# Examples

Small, runnable demonstrations of the Beacon memory engine and its MCP surface. Each
example uses only the built `beacon` binary, the Go standard library, and (for the
drivers) Python's standard library. **No model, no network, no GPU.**

| Example | What it shows | Requires |
|---|---|---|
| [memory-basic](memory-basic/README.md) | Store one approved memory and retrieve it through the real MCP server | Go, Python |
| [memory-scale](memory-scale/README.md) | Indexed retrieval over a 1000-memory store + the allocation benchmark | Go, Python |
| [agent-mcp](agent-mcp/README.md) | Wire Beacon's local MCP server into an agent runtime and prove the tool surface | Go, Python |

There are also upstream product examples that require cloud/CI infrastructure:
[fleet](fleet) (macOS S3 fleet config) and [github-actions](github-actions)
(telemetry upload workflows). Those are part of the upstream product, not the memory
optimization.

## Build once, reuse everywhere

```sh
cd cli/beacon
CGO_ENABLED=0 go build -o /tmp/beacon .      # Windows: go build -o %TEMP%\beacon.exe .
export BEACON_BIN=/tmp/beacon
```

Then run any example's `run.py`. Each one creates its own isolated temp workspace and
cleans up nothing outside it.

## What these examples establish — and do not

They prove the **interface works end-to-end** (seed → MCP → retrieve). They do **not**
measure retrieval quality or agent success. For measured results, see
[../docs/optimization/performance.md](../docs/optimization/performance.md) and
[../docs/optimization/benchmarks.md](../docs/optimization/benchmarks.md).
