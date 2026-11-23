#!/usr/bin/env python3
"""Paired V7-vs-original diagnostic on already-seen Phase 5 categories.

This is development evidence only. It cannot establish V7 superiority on a
fresh holdout. It reuses Phase 5's exact model prompt and deterministic grader.
"""
import argparse
import json
import random
import sys
import time
from pathlib import Path

PHASE5_HARNESS = Path(__file__).resolve().parents[2] / "phase5" / "harness"
sys.path.insert(0, str(PHASE5_HARNESS))
from agent_eval import SYSTEM, chat, grade  # noqa: E402


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixture", required=True)
    parser.add_argument("--selection", required=True)
    parser.add_argument("--original-results", nargs="+", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--ollama-url", default="http://192.0.2.10:11434/api/chat")
    parser.add_argument("--model", default="qwen2.5-coder:7b")
    parser.add_argument("--categories", default="A3_knowledge_transfer,B2_ambiguity,B3_multi_hop,B5_trust_adversarial")
    args = parser.parse_args()

    tasks = json.loads(Path(args.fixture).read_text(encoding="utf-8"))["tasks"]
    selected = {r["id"]: r for r in json.loads(Path(args.selection).read_text(encoding="utf-8"))}
    original = {}
    for path in args.original_results:
        for r in json.loads(Path(path).read_text(encoding="utf-8")):
            if r["arm"] == "B_original" and r["seed"] == 0:
                original[r["task"]] = r["retrieved_ids"]
    categories = set(args.categories.split(","))
    tasks = [t for t in tasks if t["category"] in categories]
    rng = random.Random(20261023)
    rng.shuffle(tasks)
    results = []
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    for task in tasks:
        by_id = {m["id"]: m for m in task["memories"]}
        arms = [("original", original[task["id"]]), ("v7", selected[task["id"]]["selected"])]
        rng.shuffle(arms)
        for arm, ids in arms:
            notes = "\n".join(f"- {by_id[i]['title']}: {by_id[i]['body']}" for i in ids if i in by_id) or "(none)"
            messages = [{"role": "system", "content": SYSTEM},
                        {"role": "user", "content": task["request"] + "\n\nProject notes:\n" + notes}]
            start = time.perf_counter()
            response = chat(args.ollama_url, args.model, 0, messages)
            elapsed = time.perf_counter() - start
            answer = (response.get("message") or {}).get("content") or ""
            result = grade(task, answer)
            results.append({"task": task["id"], "category": task["category"], "arm": arm,
                            "selected_ids": ids, "correct": result["correct"], "grade": result,
                            "answer": answer[:600], "agent_seconds": round(elapsed, 3),
                            "prompt_tokens": response.get("prompt_eval_count", 0),
                            "model": args.model, "seed": 0})
        output.write_text(json.dumps(results, indent=2) + "\n", encoding="utf-8")
        print(task["id"], flush=True)
    print(f"WROTE {output} {len(results)} runs")


if __name__ == "__main__":
    main()
