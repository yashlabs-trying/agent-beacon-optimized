#!/usr/bin/env python3
"""Phase 4B Stage C — adversarial memory agent pilot.

For each case and arm, retrieves top-K evidence, builds an identical prompt for the
same model, and grades the final answer deterministically (value / set / abstain /
source). Retrieval methods are exercised in-process (Go dump) except the lexical
CLI arms (original / v6_lexical), which are run through the real binaries.

Arms:
  none, original, v6_lexical, v6_hashed, v6_hybrid, v6_neural, oracle

Records per (case, arm): retrieved IDs, required-evidence Recall@K, whether the
answer was correct, whether a forbidden (wrong-memory) token leaked, latency, tokens,
and a failure class. Never exposes gold answers or tests to the model.
"""
import argparse
import json
import re
import subprocess
import time
import urllib.request
from pathlib import Path

ABSTAIN_PHRASES = ["cannot determine", "can't determine", "cannot be determined",
                   "not enough information", "insufficient information", "unclear",
                   "unknown", "no information", "unable to determine", "not specified",
                   "i don't know", "i do not know", "cannot say"]

SYSTEM = ("You are a software engineering assistant. Answer the question using ONLY the "
          "provided project notes. Give a direct, short answer. If the notes do not "
          "contain the answer, reply exactly: 'Cannot determine from the provided notes.'")


def norm(s):
    return re.sub(r"\s+", " ", (s or "").lower()).strip()


def grade(case, answer):
    a = norm(answer)
    if case["answer_type"] == "abstain":
        abstained = any(p in a for p in ABSTAIN_PHRASES)
        leaked = [t for t in case["forbidden_tokens"] if norm(t) in a]
        return {"correct": bool(abstained and not leaked), "abstained": abstained,
                "leaked_forbidden": leaked, "reason": "abstain"}
    gold_ok = all(norm(t) in a for t in case["gold_tokens"])
    leaked = [t for t in case["forbidden_tokens"] if norm(t) in a]
    if case["answer_type"] == "source":
        src = norm(case.get("source_token") or "")
        correct = bool(gold_ok and not leaked and src and src in a)
    else:
        correct = bool(gold_ok and not leaked)
    return {"correct": correct, "abstained": any(p in a for p in ABSTAIN_PHRASES),
            "leaked_forbidden": leaked, "gold_present": gold_ok, "reason": case["answer_type"]}


def ollama_chat(url, model, messages):
    payload = {"model": model, "messages": messages, "stream": False,
               "options": {"temperature": 0, "seed": 42, "num_ctx": 4096, "num_predict": 256}}
    req = urllib.request.Request(url, data=json.dumps(payload).encode(),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=180) as resp:
        return json.load(resp)


def load_fixture(path):
    return json.loads(Path(path).read_text(encoding="utf-8"))["cases"]


def load_retrieval(path):
    data = json.loads(Path(path).read_text(encoding="utf-8"))
    return data.get("k", 5), data["methods"]


def cli_ids(seed_bin, binary, case, workspace, db, fixture, limit):
    subprocess.run([str(seed_bin), case["id"], str(workspace),
                    str(db), str(fixture)], check=True, capture_output=True, text=True, timeout=60)
    cmd = [str(binary), "memory", "list", "--json", "--limit", str(limit),
           "--query", case["query"], "--project", str(workspace),
           "--log-path", str(workspace / "events.jsonl")]
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
    p.add_argument("--retrieval", required=True)
    p.add_argument("--run-root", required=True)
    p.add_argument("--bin-dir", required=True)
    p.add_argument("--seed", required=True)
    p.add_argument("--ollama-url", required=True)
    p.add_argument("--model", default="qwen2.5-coder:7b")
    p.add_argument("--limit", type=int, default=5)
    p.add_argument("--arms", default="none,original,v6_lexical,v6_hashed,v6_hybrid,v6_neural,oracle")
    p.add_argument("--output", required=True)
    args = p.parse_args()

    cases = load_fixture(args.fixture)
    _, methods = load_retrieval(args.retrieval)
    arms = [a.strip() for a in args.arms.split(",") if a.strip()]
    root = Path(args.run_root)
    root.mkdir(parents=True, exist_ok=True)
    binf = Path(args.bin_dir)

    results = []
    for case in cases:
        by_id = {m["id"]: m for m in case["memories"]}
        retrieval = {}
        # lexical CLI arms
        if "original" in arms or "v6_lexical" in arms:
            for arm, binary in (("original", binf / "beacon-original"), ("v6_lexical", binf / "beacon-v6")):
                if arm not in arms:
                    continue
                ws = root / case["id"] / arm
                ws.mkdir(parents=True, exist_ok=True)
                try:
                    ids, ms = cli_ids(args.seed, binary, case, ws, ws / "memory.db", Path(args.fixture), args.limit)
                except Exception as exc:  # noqa: BLE001
                    ids, ms = [], -1.0
                    print(f"WARN {case['id']}/{arm}: {exc}")
                retrieval[arm] = (ids, ms)
        for arm in ("v6_lexical", "v6_hashed", "v6_hybrid", "v6_neural"):
            if arm in arms and arm not in retrieval:
                key = arm.replace("v6_", "")
                key = "lexical" if key == "lexical" else key
                ids = methods.get(case["id"], {}).get(key, [])
                retrieval[arm] = (ids, -1.0)
        if "none" in arms:
            retrieval["none"] = ([], 0.0)
        if "oracle" in arms:
            retrieval["oracle"] = (list(case["required_evidence"]), 0.0)

        for arm in arms:
            ids, ret_ms = retrieval.get(arm, ([], 0.0))
            ids = [i for i in ids if i in by_id][: args.limit]
            notes = "\n".join(f"- {by_id[i]['title']}: {by_id[i]['body']}" for i in ids) or "(none)"
            messages = [{"role": "system", "content": SYSTEM},
                        {"role": "user", "content": case["query"] + "\n\nProject notes:\n" + notes}]
            start = time.perf_counter()
            resp = ollama_chat(args.ollama_url, args.model, messages)
            elapsed = time.perf_counter() - start
            answer = (resp.get("message", {}) or {}).get("content", "") or ""
            g = grade(case, answer)
            req = set(case["required_evidence"])
            recall = len(req & set(ids)) / len(req) if req else 1.0
            results.append({
                "case": case["id"], "category": case["category"], "difficulty": case["difficulty"],
                "arm": arm, "retrieved_ids": ids, "required_evidence": case["required_evidence"],
                "required_evidence_recall": round(recall, 3),
                "answer": answer.strip()[:500], "correct": g["correct"], "grade": g,
                "forbidden_leaked": g["leaked_forbidden"],
                "retrieval_ms": round(ret_ms, 3), "agent_seconds": round(elapsed, 3),
                "prompt_tokens": resp.get("prompt_eval_count", 0),
                "gen_tokens": resp.get("eval_count", 0),
            })
            print(f"{case['id']}/{arm}: correct={g['correct']} recall={recall:.2f} "
                  f"leaked={g['leaked_forbidden']} t={elapsed:.1f}s", flush=True)

    Path(args.output).write_text(json.dumps(results, indent=2) + "\n", encoding="utf-8")
    print(f"WROTE {args.output} runs={len(results)}")


if __name__ == "__main__":
    main()
