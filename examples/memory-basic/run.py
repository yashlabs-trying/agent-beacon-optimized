#!/usr/bin/env python3
"""Example A — basic memory round-trip through the real Beacon MCP server.

Steps: build a tiny in-repo seeder that calls the real Store.PutMemory, seed one
approved memory, then drive `beacon mcp serve` over stdio and assert the memory
comes back from `search_memory`.

Env:
  BEACON_BIN   path to the beacon CLI (default: /tmp/beacon on POSIX, beacon.exe on Windows)
Run from the repository root or this directory.
"""
import json, os, subprocess, sys, tempfile, shutil

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
CLI = os.path.join(REPO, "cli", "beacon")
DEFAULT_BIN = "beacon.exe" if os.name == "nt" else "/tmp/beacon"

SEED_MAIN = '''package main

import (
    "encoding/json"
    "fmt"
    "os"

    "github.com/asymptote-labs/agent-beacon/cli/beacon/internal/learning"
    "github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// Minimal seeder: writes one approved memory through the real Store API.
func main() {
    db, spec, dir := os.Args[1], os.Args[2], os.Args[3]
    raw, err := os.ReadFile(spec)
    if err != nil { panic(err) }
    var recs []asymptoteobserve.LearningMemoryV1
    if err := json.Unmarshal(raw, &recs); err != nil { panic(err) }
    pid := ""
    if p, perr := learning.ResolveProject(dir); perr == nil { pid = p.ID }
    s := learning.Open(db)
    for i := range recs {
        if recs[i].Project.ID == "" { recs[i].Project.ID = pid }
        if err := s.PutMemory(recs[i]); err != nil { panic(err) }
    }
    fmt.Printf("PROJECTID=%s\\n", pid)
}
'''

MEM = [{
    "id": "ex-a-1", "candidate_id": "c-ex-a-1", "kind": "fact",
    "title": "Service port",
    "body": "The service port is 8443.",
    "project": {"id": ""},
}]


def rpc(proc, msg):
    proc.stdin.write(json.dumps(msg) + "\n")
    proc.stdin.flush()


def read_response(proc, want_id, timeout_lines=2000):
    for _ in range(timeout_lines):
        line = proc.stdout.readline()
        if not line:
            break
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            ev = json.loads(line)
        except Exception:
            continue
        if ev.get("id") == want_id:
            return ev
    return None


def main():
    beacon = os.environ.get("BEACON_BIN", DEFAULT_BIN)
    if not (shutil.which(beacon) or os.path.exists(beacon)):
        print(f"FAIL: beacon binary not found ({beacon}); set BEACON_BIN", file=sys.stderr)
        return 2

    ws = tempfile.mkdtemp(prefix="beacon-example-A-")
    print("workspace:", ws)
    os.makedirs(os.path.join(ws, "endpoint", "logs"), exist_ok=True)

    # Build a seeder from the in-repo sources (uses the real Store.PutMemory).
    seeder_dir = os.path.join(ws, "seedtool")
    os.makedirs(seeder_dir, exist_ok=True)
    open(os.path.join(seeder_dir, "main.go"), "w").write(SEED_MAIN)
    seed_bin = os.path.join(ws, "seedtool.exe" if os.name == "nt" else "seedtool")
    # Place the seeder inside the module so it can import internal packages.
    tool_dir = os.path.join(CLI, "cmd", "zz_example_seedtool")
    os.makedirs(tool_dir, exist_ok=True)
    open(os.path.join(tool_dir, "main.go"), "w").write(SEED_MAIN)
    env = dict(os.environ, CGO_ENABLED="0")
    build = subprocess.run(["go", "build", "-o", seed_bin, "./cmd/zz_example_seedtool"],
                           cwd=CLI, capture_output=True, text=True, env=env)
    shutil.rmtree(tool_dir, ignore_errors=True)
    if build.returncode != 0:
        print("FAIL: seeder build failed:\n" + build.stderr[-800:], file=sys.stderr)
        return 2

    spec = os.path.join(ws, "seed.json")
    json.dump(MEM, open(spec, "w"))
    db = os.path.join(ws, "endpoint", "memory.db")
    seeded = subprocess.run([seed_bin, db, spec, ws], capture_output=True, text=True, env=env)
    pid = ""
    for line in seeded.stdout.splitlines():
        if line.startswith("PROJECTID="):
            pid = line.split("=", 1)[1].strip()
    if not pid:
        print("FAIL: seed step did not report a project id:\n" + seeded.stderr[-400:], file=sys.stderr)
        return 2
    print("seed:      OK (project %s...)" % pid[:22])

    log = os.path.join(ws, "endpoint", "logs", "runtime.jsonl")
    cmd = [beacon, "mcp", "serve", "--transport", "stdio", "--log-path", log,
           "--project-allowlist", pid]
    proc = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, text=True, cwd=ws)

    rpc(proc, {"jsonrpc": "2.0", "id": 1, "method": "initialize",
               "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                          "clientInfo": {"name": "example-a", "version": "1"}}})
    init = read_response(proc, 1)
    rpc(proc, {"jsonrpc": "2.0", "method": "notifications/initialized"})
    rpc(proc, {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}})
    tools_ev = read_response(proc, 2)
    tools = [t.get("name") for t in tools_ev["result"]["tools"]] if tools_ev else []
    print("tools:     " + ", ".join(tools))

    rpc(proc, {"jsonrpc": "2.0", "id": 3, "method": "tools/call",
               "params": {"name": "search_memory", "arguments": {"query": "service port"}}})
    res = read_response(proc, 3)
    proc.terminate()
    blob = json.dumps(res)
    print("search_memory -> " + blob[:240])
    ok = res is not None and "8443" in blob
    print("RESULT: " + ("PASS — the seeded memory round-tripped through MCP." if ok
                        else "FAIL — seeded memory not returned."))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
