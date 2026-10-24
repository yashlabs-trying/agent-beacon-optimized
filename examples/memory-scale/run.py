#!/usr/bin/env python3
"""Example B — large-scale retrieval through the real Beacon MCP server.

Seeds N distinct memories (one is the needle) via the real Store.PutMemory, then
drives `beacon mcp serve` and asserts the needle is returned by `search_memory`.

Env:
  BEACON_BIN   path to the beacon CLI
Usage:
  python3 run.py [--count 1000]
"""
import argparse, json, os, subprocess, sys, tempfile, shutil, time

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
CLI = os.path.join(REPO, "cli", "beacon")
DEFAULT_BIN = "beacon.exe" if os.name == "nt" else "/tmp/beacon"

SEED_MAIN = '''package main

import (
    "encoding/json"
    "fmt"
    "os"
    "strconv"

    "github.com/asymptote-labs/agent-beacon/cli/beacon/internal/learning"
    "github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func main() {
    db, dir := os.Args[1], os.Args[2]
    n, _ := strconv.Atoi(os.Args[3])
    pid := ""
    if p, perr := learning.ResolveProject(dir); perr == nil { pid = p.ID }
    s := learning.Open(db)
    for i := 0; i < n; i++ {
        rec := asymptoteobserve.LearningMemoryV1{
            ID: fmt.Sprintf("mem-%d", i), CandidateID: fmt.Sprintf("c-%d", i),
            Kind: "note", Title: fmt.Sprintf("Historical note %d", i),
            Body: fmt.Sprintf("Routine historical note number %d; no decision recorded.", i),
            Project: asymptoteobserve.LearningProjectV1{ID: pid},
        }
        if err := s.PutMemory(rec); err != nil { panic(err) }
    }
    needle := asymptoteobserve.LearningMemoryV1{
        ID: "needle", CandidateID: "c-needle", Kind: "fact",
        Title: "Deployment key", Body: "The correct deployment key for this project is KEY-Z7Q9.",
        Project: asymptoteobserve.LearningProjectV1{ID: pid},
    }
    if err := s.PutMemory(needle); err != nil { panic(err) }
    raw, _ := json.Marshal(map[string]string{"project": pid})
    fmt.Println(string(raw))
}
'''


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--count", type=int, default=1000)
    args = ap.parse_args()

    beacon = os.environ.get("BEACON_BIN", DEFAULT_BIN)
    if not (shutil.which(beacon) or os.path.exists(beacon)):
        print(f"FAIL: beacon not found ({beacon}); set BEACON_BIN", file=sys.stderr)
        return 2

    ws = tempfile.mkdtemp(prefix="beacon-example-B-")
    print("workspace:", ws)
    os.makedirs(os.path.join(ws, "endpoint", "logs"), exist_ok=True)

    tool_dir = os.path.join(CLI, "cmd", "zz_example_b_seedtool")
    os.makedirs(tool_dir, exist_ok=True)
    open(os.path.join(tool_dir, "main.go"), "w").write(SEED_MAIN)
    seed_bin = os.path.join(ws, "seedtool.exe" if os.name == "nt" else "seedtool")
    env = dict(os.environ, CGO_ENABLED="0")
    build = subprocess.run(["go", "build", "-o", seed_bin, "./cmd/zz_example_b_seedtool"],
                           cwd=CLI, capture_output=True, text=True, env=env)
    shutil.rmtree(tool_dir, ignore_errors=True)
    if build.returncode != 0:
        print("FAIL: seeder build:\n" + build.stderr[-800:], file=sys.stderr)
        return 2

    db = os.path.join(ws, "endpoint", "memory.db")
    t0 = time.time()
    seeded = subprocess.run([seed_bin, db, ws, str(args.count)],
                            capture_output=True, text=True, env=env)
    if seeded.returncode != 0:
        print("FAIL: seed:\n" + seeded.stderr[-400:], file=sys.stderr)
        return 2
    pid = json.loads(seeded.stdout.strip().splitlines()[-1]).get("project", "")
    print("seeded:    %d memories (+ needle)" % args.count)

    log = os.path.join(ws, "endpoint", "logs", "runtime.jsonl")
    cmd = [beacon, "mcp", "serve", "--transport", "stdio", "--log-path", log,
           "--project-allowlist", pid]
    proc = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, text=True, cwd=ws)

    def rpc(m):
        proc.stdin.write(json.dumps(m) + "\n"); proc.stdin.flush()

    def read(want):
        for _ in range(5000):
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
                    "clientInfo": {"name": "example-b", "version": "1"}}})
    read(1)
    rpc({"jsonrpc": "2.0", "method": "notifications/initialized"})
    t1 = time.time()
    rpc({"jsonrpc": "2.0", "id": 2, "method": "tools/call",
         "params": {"name": "search_memory", "arguments": {"query": "deployment key"}}})
    res = read(2)
    elapsed_ms = (time.time() - t1) * 1000
    proc.terminate()
    blob = json.dumps(res)
    print("search_memory (needle) -> " + blob[:200])
    print("elapsed:   %.1f ms for a %d-memory store" % (elapsed_ms, args.count))
    ok = res is not None and "KEY-Z7Q9" in blob
    print("RESULT: " + ("PASS — indexed retrieval returned the needle." if ok
                        else "FAIL — needle not returned."))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
