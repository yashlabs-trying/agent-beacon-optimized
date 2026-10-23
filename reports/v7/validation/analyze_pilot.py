#!/usr/bin/env python3
"""Summarize paired local-model runs; preserve the raw file separately."""
import argparse
import json
import statistics
from collections import Counter, defaultdict
from pathlib import Path


def main():
    p = argparse.ArgumentParser()
    p.add_argument("input")
    args = p.parse_args()
    rows = json.loads(Path(args.input).read_text(encoding="utf-8"))["results"]
    by_task = defaultdict(dict)
    for row in rows:
        by_task[row["task"]][row["arm"]] = row
    families = defaultdict(list)
    losses, wins = [], []
    for task, arms in by_task.items():
        if set(arms) != {"original", "v7"}:
            raise SystemExit(f"incomplete task {task}: {sorted(arms)}")
        o, v = arms["original"], arms["v7"]
        families[o["category"]].append((o, v))
        if o["correct"] and not v["correct"]:
            losses.append(task)
        if v["correct"] and not o["correct"]:
            wins.append(task)
    print("Task count", len(by_task), "runs", len(rows))
    print("Family | Original | V7 | wins | losses | V7 evidence recall")
    for family, pairs in sorted(families.items()):
        evidence = sum(set(v["required_evidence"]) <= set(v["selected_ids"]) for _, v in pairs)
        print(family, sum(o["correct"] for o, _ in pairs),
              sum(v["correct"] for _, v in pairs),
              sum(v["correct"] and not o["correct"] for o, v in pairs),
              sum(o["correct"] and not v["correct"] for o, v in pairs),
              f"{evidence}/{len(pairs)}")
    print("Total", sum(r["correct"] for r in rows if r["arm"] == "original"),
          sum(r["correct"] for r in rows if r["arm"] == "v7"))
    print("Wins", wins)
    print("Losses", losses)
    for arm in ("original", "v7"):
        arm_rows = [r for r in rows if r["arm"] == arm]
        for key in ("retrieval_seconds", "agent_seconds", "end_to_end_seconds", "prompt_tokens"):
            vals = [r[key] for r in arm_rows]
            print(arm, key, "mean", round(statistics.mean(vals), 4),
                  "p50", round(statistics.median(vals), 4),
                  "p95", round(sorted(vals)[min(len(vals)-1, int(.95*len(vals)))], 4),
                  "max", round(max(vals), 4))
    print("V7 routes", Counter(r["diagnostics"].get("route") for r in rows if r["arm"] == "v7"))
    for task in losses:
        o, v = by_task[task]["original"], by_task[task]["v7"]
        print("LOSS", task, "original IDs", o["selected_ids"], "v7 IDs", v["selected_ids"],
              "original answer", repr(o["answer"]), "v7 answer", repr(v["answer"]))


if __name__ == "__main__":
    main()
