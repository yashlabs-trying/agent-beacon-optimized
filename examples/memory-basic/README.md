# Example A — Basic memory round-trip

Store one approved memory and retrieve it with the **real** local Beacon MCP server
(`beacon mcp serve`). This is the smallest end-to-end demonstration: it seeds a SQLite
memory store, speaks JSON-RPC to the MCP server over stdio, calls `search_memory`, and
asserts the stored fact comes back.

No model, no network, no GPU.

## What you need

- Go 1.27+ (to build the CLI) **or** a prebuilt `beacon` binary on your `PATH`.
- Python 3.10+ (for the driver script; standard library only).

## Step 1 — build the CLI

```sh
cd cli/beacon
CGO_ENABLED=0 go build -o /tmp/beacon .
```

## Step 2 — run the example

```sh
cd examples/memory-basic
BEACON_BIN=/tmp/beacon python3 run.py
```

`run.py` does four things, all against the real binary:

1. Creates an isolated workspace with an empty SQLite store.
2. Seeds one approved memory directly through Beacon's own store API — it builds and
   runs a tiny Go helper (`seedtool`) that calls `learning.Store.PutMemory`, the same
   path approval uses.
3. Starts `beacon mcp serve --transport stdio` scoped to that store and sends
   `initialize` + `tools/call` for `search_memory`.
4. Asserts the seeded fact is returned.

## Expected output

```
workspace: /tmp/beacon-example-A-XXXX
seed:      OK (project sha256:...)
tools:     search_activity, summarize_activity, get_activity_event,
           list_activity_filters, search_memory, get_memory, get_memory_context
search_memory -> {"memories":[{"id":"ex-a-1","title":"Service port","body":"The service port is 8443."}]}
RESULT: PASS — the seeded memory round-tripped through MCP.
```

## Troubleshooting

- **`beacon: command not found`** — set `BEACON_BIN` to the binary you built.
- **`search_memory` returns `{"memories":[]}`** — confirm the seed step printed
  `seed: OK`. If you point `--project-allowlist` at a different project than the one
  seeded, the record is intentionally invisible (that is the authorization feature).
- **Windows** — the driver works on Windows too (it uses the MCP stdio transport); the
  example prints POSIX-style paths for readability.

See [../../docs/optimization/architecture.md](../../docs/optimization/architecture.md)
for what `search_memory` does under the hood.
