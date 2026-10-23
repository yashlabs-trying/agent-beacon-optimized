#!/usr/bin/env python3
"""Phase 4C agent pilot: compares memory systems and isolates each context-selection
mechanism. Same model, prompt, K, and grading for every arm; only the memory
evidence differs.

Arms:
  none, original (CLI lexical), v6_semantic (raw semantic retrieval),
  v6_intelligent (semantic + all selection mechanisms), oracle,
  ab_supersession, ab_trust, ab_conflict, ab_injection (isolated mechanisms).
"""
import argparse
import json
import re
import subprocess
import time
import urllib.request
from pathlib import Path

ABSTAIN = ["cannot determine", "can't determine", "cannot be determined", "not enough",
           "insufficient", "unclear", "unknown", "no information", "unable to determine",
           "not specified", "i don't know", "i do not know", "cannot say"]

SYSTEM = ("You are a software engineering assistant. Answer the question using ONLY the "
          "provided project notes. Give a direct, short answer. If the notes do not "
          "contain a reliable answer, reply exactly: 'Cannot determine from the provided notes.'")

ARM_TO_KEY = {
    "v6_semantic": ("retrieved", "semantic"),
    "v6_intelligent": ("configs", "plus_all"),
    "ab_supersession": ("configs", "plus_supersession"),
    "ab_trust": ("configs", "plus_trust"),
    "ab_conflict": ("configs", "plus_conflict"),
    "ab_injection": ("configs", "plus_injection"),
}


def norm(s):
    return re.sub(r"\s+", " ", (s or "").lower()).strip()


def grade(case, answer):
    a = norm(answer)
    if case["answer_type"] == "abstain":
        abstained = any(p in a for p in ABSTAIN)
        leaked = [t for t in case["forbidden_tokens"] if norm(t) in a]
        return {"correct": bool(abstained and not leaked), "abstained": abstained, "leaked": leaked}
    gold_ok = all(norm(t) in a for t in case["gold_tokens"])
    leaked = [t for t in case["forbidden_tokens"] if norm(t) in a]
    return {"correct": bool(gold_ok and not leaked), "abstained": any(p in a for p in ABSTAIN),
            "gold_present": gold_ok, "leaked": leaked}


def chat(url, model, messages):
    payload = {"model": model, "messages": messages, "stream": False,
               "options": {"temperature": 0, "seed": 42, "num_ctx": 4096, "num_predict": 256}}
    req = urllib.request.Request(url, data=json.dumps(payload).encode(),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=180) as resp:
        return json.load(resp)


def cli_ids(seed_bin, binary, case, workspace, db, fixture, limit):
    subprocess.run([str(seed_bin), case["id"], str(workspace), str(db), str(fixture)],
                   check=True, capture_output=True, text=True, timeout=60)
    cmd = [str(binary), "memory", "list", "--json", "--limit", str(limit), "--query", case["query"],
           "--project", str(workspace), "--log-path", str(workspace / "events.jsonl")]
    start = time.perf_counter()
    out = subprocess.run(cmd, check=True, capture_output=True, text=True, timeout=60)
    ms = (time.perf_counter() - start) * 1000
    try:
        return [m.get("id") for m in json.loads(out.stdout)], ms
    except json.JSONDecodeError:
        return [], ms


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--fixture", required=True)
    p.add_argument("--selection", required=True)
    p.add_argument("--run-root", required=True)
    p.add_argument("--bin-dir", required=True)
    p.add_argument("--seed", required=True)
    p.add_argument("--ollama-url", required=True)
    p.add_argument("--model", default="qwen2.5-coder:7b")
    p.add_argument("--limit", type=int, default=8)
    p.add_argument("--arms", default="none,original,v6_semantic,v6_intelligent,oracle,"
                                    "ab_supersession,ab_trust,ab_conflict,ab_injection")
    p.add_argument("--output", required=True)
    args = p.parse_args()

    cases = json.loads(Path(args.fixture).read_text(encoding="utf-8"))["cases"]
    sel = json.loads(Path(args.selection).read_text(encoding="utf-8"))["cases"]
    arms = [a.strip() for a in args.arms.split(",") if a.strip()]
    root = Path(args.run_root)
    root.mkdir(parents=True, exist_ok=True)
    binf = Path(args.bin_dir)
    results = []

    for case in cases:
        by_id = {m["id"]: m for m in case["memories"]}
        sc = sel.get(case["id"], {"retrieved": {}, "configs": {}})
        retrieval = {}
        if "original" in arms:
            ws = root / case["id"] / "original"
            ws.mkdir(parents=True, exist_ok=True)
            try:
                retrieval["original"] = cli_ids(args.seed, binf / "beacon-original", case, ws,
                                                ws / "memory.db", Path(args.fixture), args.limit)
            except Exception as exc:  # noqa: BLE001
                print(f"WARN {case['id']}/original: {exc}")
                retrieval["original"] = ([], -1.0)
        if "none" in arms:
            retrieval["none"] = ([], 0.0)
        if "oracle" in arms:
            retrieval["oracle"] = (list(case["required_evidence"]), 0.0)
        for arm, (section, key) in ARM_TO_KEY.items():
            if arm in arms:
                retrieval[arm] = (sc.get(section, {}).get(key, []), -1.0)

        for arm in arms:
            ids, ret_ms = retrieval.get(arm, ([], 0.0))
            ids = ids or []
            ids = [i for i in ids if i in by_id][: args.limit]
            notes = "\n".join(f"- {by_id[i]['title']}: {by_id[i]['body']}" for i in ids) or "(none)"
            messages = [{"role": "system", "content": SYSTEM},
                        {"role": "user", "content": case["query"] + "\n\nProject notes:\n" + notes}]
            start = time.perf_counter()
            resp = chat(args.ollama_url, args.model, messages)
            elapsed = time.perf_counter() - start
            answer = (resp.get("message", {}) or {}).get("content", "") or ""
            g = grade(case, answer)
            req = set(case["required_evidence"])
            recall = len(req & set(ids)) / len(req) if req else 1.0
            results.append({
                "case": case["id"], "category": case["category"], "subcategory": case["subcategory"],
                "difficulty": case["difficulty"], "arm": arm, "retrieved_ids": ids,
                "required_evidence": case["required_evidence"], "evidence_recall": round(recall, 3),
                "answer": answer.strip()[:400], "correct": g["correct"], "grade": g,
                "forbidden_leaked": g["leaked"], "retrieval_ms": round(ret_ms, 3),
                "agent_seconds": round(elapsed, 3), "prompt_tokens": resp.get("prompt_eval_count", 0),
                "gen_tokens": resp.get("eval_count", 0),
            })
            print(f"{case['id']}/{arm}: correct={g['correct']} recall={recall:.2f} "
                  f"leaked={g['leaked']}", flush=True)

    Path(args.output).write_text(json.dumps(results, indent=2) + "\n", encoding="utf-8")
    print(f"WROTE {args.output} runs={len(results)}")


if __name__ == "__main__":
    main()
