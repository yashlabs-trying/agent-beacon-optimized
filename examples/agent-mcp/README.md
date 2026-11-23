# Example C — Agent integration (MCP)

Wire Beacon's local memory server into an agent runtime so the agent can call
`search_memory`, `get_memory`, and `get_memory_context` as MCP tools.

This example does **not** require a model account. It shows the exact configuration an
agent uses, then proves the server works by listing tools and calling one over stdio.

## 1. Configure the agent

Beacon exposes a **local** MCP server, separate from the hosted Beacon Cloud MCP. Point
your runtime at it with the standard `mcp` block. For a project scoped to its own memory
store:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "beacon": {
      "type": "local",
      "enabled": true,
      "command": [
        "/tmp/beacon",
        "mcp",
        "serve",
        "--transport", "stdio",
        "--log-path", "./endpoint/logs/runtime.jsonl",
        "--project-allowlist", "<project-id>"
      ]
    }
  }
}
```

- Any MCP-capable client works (OpenCode, Claude Code, Cursor, Codex CLI, …). The block
  above is the OpenCode form; other runtimes use the same command array under their own
  MCP key.
- `--project-allowlist <id>` scopes the server to one project so `search_memory` cannot
  read other projects. Omit it only for the documented single-user local default.
- The server never touches the network.

## 2. Prove it works without a model

```sh
cd examples/agent-mcp
BEACON_BIN=/tmp/beacon python3 run.py
```

`run.py` starts `beacon mcp serve` over stdio exactly as an agent would, performs the
MCP handshake, and lists the tools the agent will see:

```
initialize      -> {"serverInfo":{"name":"beacon",...}}
notifications/initialized
tools/list      -> search_activity, summarize_activity, get_activity_event,
                   list_activity_filters, search_memory, get_memory, get_memory_context
tools/call search_memory -> {"memories":[...]}
RESULT: PASS — the agent-facing MCP surface is live.
```

## 3. What the agent gains

Once wired, the agent can, mid-task:

- `search_memory` — lexical-first, question-aware retrieval over approved project memory
  (see [../../docs/optimization/retrieval-pipeline.md](../../docs/optimization/retrieval-pipeline.md)).
- `get_memory` — fetch one memory by id (project-scoped).
- `get_memory_context` — the assembled, trust/supersession-checked context block.

This is what makes "a problem solved by one agent not need re-learning by another"
concrete across harnesses.

## Troubleshooting

| Symptom | Fix |
|---|---|
| Agent shows no `beacon` tools | Confirm `beacon mcp serve --help` exists in your binary, and that the `command` path is absolute. |
| Tools exist but `search_memory` is empty | No approved memory yet, or the project allowlist does not match the store's project. |
| `beacon: not found` | Use the absolute path to the built binary in `command`. |

See [../../docs/optimization/configuration.md](../../docs/optimization/configuration.md)
for placement and scoping, and [../../SECURITY.md](../../SECURITY.md) for the trust model.
