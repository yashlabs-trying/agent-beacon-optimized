#!/usr/bin/env python3
"""Example C — agent/MCP integration handshake.

Starts `beacon mcp serve` over stdio exactly as an agent runtime would, performs the
MCP handshake, lists the tools the agent will see, and makes one tool call.

Env:
  BEACON_BIN   path to the beacon CLI
"""
import json, os, subprocess, sys, tempfile, shutil

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_BIN = "beacon.exe" if os.name == "nt" else "/tmp/beacon"


def main():
    beacon = os.environ.get("BEACON_BIN", DEFAULT_BIN)
    if not (shutil.which(beacon) or os.path.exists(beacon)):
        print(f"FAIL: beacon not found ({beacon}); set BEACON_BIN", file=sys.stderr)
        return 2

    ws = tempfile.mkdtemp(prefix="beacon-example-C-")
    os.makedirs(os.path.join(ws, "endpoint", "logs"), exist_ok=True)
    log = os.path.join(ws, "endpoint", "logs", "runtime.jsonl")
    print("workspace:", ws)

    cmd = [beacon, "mcp", "serve", "--transport", "stdio", "--log-path", log]
    proc = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, text=True, cwd=ws)

    def rpc(m):
        proc.stdin.write(json.dumps(m) + "\n"); proc.stdin.flush()

    def read(want):
        for _ in range(3000):
            line = proc.stdout.readline()
            if not line:
                return None
            line = line.strip()
            if not line.startswith("{"):
                continue
            try:
                ev = json.loads(line)
            except Exception:
                continue
            if ev.get("id") == want:
                return ev
        return None

    rpc({"jsonrpc": "2.0", "id": 1, "method": "initialize",
         "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                    "clientInfo": {"name": "example-c", "version": "1"}}})
    init = read(1)
    name = (init or {}).get("result", {}).get("serverInfo", {}).get("name", "?")
    print(f"initialize      -> serverInfo.name={name}")
    rpc({"jsonrpc": "2.0", "method": "notifications/initialized"})

    rpc({"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}})
    tev = read(2)
    tools = [t.get("name") for t in (tev or {}).get("result", {}).get("tools", [])]
    print("tools/list      -> " + ", ".join(tools))

    rpc({"jsonrpc": "2.0", "id": 3, "method": "tools/call",
         "params": {"name": "search_memory", "arguments": {"query": "anything"}}})
    res = read(3)
    proc.terminate()
    print("tools/call search_memory -> " + (json.dumps(res)[:120] if res else "no response"))
    ok = name == "beacon" and "search_memory" in tools and res is not None
    print("RESULT: " + ("PASS — the agent-facing MCP surface is live." if ok
                        else "FAIL — MCP surface incomplete."))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
