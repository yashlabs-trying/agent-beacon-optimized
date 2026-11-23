#!/usr/bin/env python3
"""Run the locked V7 pilot with real original and V7 CLIs on one seeded store.

The committed fixture and V7 candidate predate this script's first model call.
Results are written after every task so an interrupted local run is recoverable.
"""
import argparse
import hashlib
import json
import random
import subprocess
import sys
import time
from pathlib import Path

PHASE5_HARNESS = Path(__file__).resolve().parents[2] / "phase5" / "harness"
sys.path.insert(0, str(PHASE5_HARNESS))
from agent_eval import SYSTEM, chat, grade  # noqa: E402

FIXTURE_SHA256 = "0c55552721e1ae0dedb979bd9eabdced6e03eeb440ca38e7bb7209d7013ad044"


def run_cli(binary, task, workspace, v7, semantic=True):
    cmd = [str(binary), "memory", "list", "--json", "--limit", "8",
           "--query", task["retrieval_query"], "--project", str(workspace),
           "--log-path", str(workspace / "events.jsonl")]
    if v7:
        cmd += ["--question-aware", "--question", task["request"]]
        if semantic:
            cmd += ["--semantic"]
    start = time.perf_counter()
    proc = subprocess.run(cmd, check=True, capture_output=True, text=True, timeout=60)
    elapsed = time.perf_counter() - start
    data = json.loads(proc.stdout)
    if v7:
        items = (data.get("context") or {}).get("Items") or []
        ids = [item["Memory"]["id"] for item in items]
        diagnostics = {k: data.get(k) for k in ("route", "embedding_model", "signals",
                                                "lexical_candidates", "semantic_candidates")}
    else:
        ids = [item["id"] for item in data]
        diagnostics = {}
    return ids, elapsed, diagnostics


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--fixture", required=True)
    p.add_argument("--run-root", required=True)
    p.add_argument("--seed-bin", required=True)
    p.add_argument("--original-bin", required=True)
    p.add_argument("--v7-bin", required=True)
    p.add_argument("--output", required=True)
    p.add_argument("--ollama-url", default="http://192.0.2.10:11434/api/chat")
    p.add_argument("--model", default="qwen2.5-coder:7b")
    p.add_argument("--v7-lexical-only", action="store_true",
                   help="Disable V7 semantic expansion for a component ablation")
    args = p.parse_args()

    fixture = Path(args.fixture)
    actual_hash = hashlib.sha256(fixture.read_bytes()).hexdigest()
    if actual_hash != FIXTURE_SHA256:
        raise SystemExit(f"holdout hash mismatch: {actual_hash}")
    tasks = json.loads(fixture.read_text(encoding="utf-8"))["tasks"]
    rng = random.Random(20261023)
    rng.shuffle(tasks)
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    if output.exists():
        raise SystemExit(f"output already exists: {output}")
    run_root = Path(args.run_root)
    run_root.mkdir(parents=True, exist_ok=True)
    results = []
    for task in tasks:
        workspace = run_root / task["id"]
        workspace.mkdir(parents=True, exist_ok=True)
        db = workspace / "memory.db"
        subprocess.run([args.seed_bin, task["id"], str(workspace), str(db), str(fixture)],
                       check=True, capture_output=True, text=True, timeout=60)
        # Original runs before V7's optional embedding backfill modifies derived rows.
        original_ids, original_retrieval, _ = run_cli(args.original_bin, task, workspace, False)
        v7_ids, v7_retrieval, v7_diagnostics = run_cli(
            args.v7_bin, task, workspace, True, not args.v7_lexical_only)
        by_id = {m["id"]: m for m in task["memories"]}
        arms = [("original", original_ids, original_retrieval, {}),
                ("v7", v7_ids, v7_retrieval, v7_diagnostics)]
        rng.shuffle(arms)
        for arm, ids, retrieval_seconds, diagnostics in arms:
            unknown = set(ids) - set(by_id)
            if unknown:
                raise RuntimeError(f"unknown selected IDs: {sorted(unknown)}")
            notes = "\n".join(f"- {by_id[i]['title']}: {by_id[i]['body']}" for i in ids) or "(none)"
            messages = [{"role": "system", "content": SYSTEM},
                        {"role": "user", "content": task["request"] + "\n\nProject notes:\n" + notes}]
            start = time.perf_counter()
            response = chat(args.ollama_url, args.model, 0, messages)
            agent_seconds = time.perf_counter() - start
            answer = (response.get("message") or {}).get("content") or ""
            result = grade(task, answer)
            results.append({"task": task["id"], "category": task["category"], "arm": arm,
                            "selected_ids": ids, "required_evidence": task["required_evidence"],
                            "correct": result["correct"], "grade": result, "answer": answer,
                            "retrieval_seconds": round(retrieval_seconds, 4),
                            "agent_seconds": round(agent_seconds, 4),
                            "end_to_end_seconds": round(retrieval_seconds + agent_seconds, 4),
                            "prompt_tokens": response.get("prompt_eval_count", 0),
                            "model": args.model, "seed": 0, "diagnostics": diagnostics})
        output.write_text(json.dumps({"fixture_sha256": actual_hash,
                                      "results": results}, indent=2) + "\n", encoding="utf-8")
        print(task["id"], flush=True)
    print(f"WROTE {output}: {len(results)} arm runs")


if __name__ == "__main__":
    main()
