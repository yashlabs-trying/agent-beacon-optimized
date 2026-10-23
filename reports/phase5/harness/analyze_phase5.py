#!/usr/bin/env python3
"""Phase 5 analysis: per-arm/per-suite/per-category accuracy, paired McNemar,
evidence recall, latency, and failure taxonomy from the raw run traces."""
import argparse
import json
import math
from collections import defaultdict
from pathlib import Path

ARMS = ["A_none", "B_original", "C_semantic", "D_intelligent", "E_oracle"]
SUITES = ["A", "B", "C"]


def binom(k, n, p=0.5):
    if n == 0:
        return 1.0
    probs = [math.comb(n, i) * p ** i * (1 - p) ** (n - i) for i in range(n + 1)]
    return min(1.0, sum(pr for pr in probs if pr <= probs[k] + 1e-12))


def wilson(k, n, z=1.96):
    if n == 0:
        return (0.0, 0.0)
    p = k / n; d = 1 + z * z / n; c = p + z * z / (2 * n)
    m = z * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n))
    return ((c - m) / d, (c + m) / d)


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--inputs", nargs="+", required=True)
    p.add_argument("--output", required=True)
    p.add_argument("--text", required=True)
    args = p.parse_args()
    runs = []
    for f in args.inputs:
        runs += json.loads(Path(f).read_text(encoding="utf-8"))
    by_arm = defaultdict(list)
    for r in runs:
        by_arm[r["arm"]].append(r)
    # aggregate per (task, arm): any seed correct
    task_arm = defaultdict(lambda: defaultdict(list))
    for r in runs:
        task_arm[r["task"]][r["arm"]].append(r["correct"])
    tasks = sorted(task_arm)
    correct = {(t, a): (sum(v) / len(v) >= 0.5) for t, arms in task_arm.items() for a, v in arms.items()}
    cats = sorted({r["category"] for r in runs})
    report = {"runs": len(runs), "tasks": len(tasks), "arms": {}, "paired": {}, "categories": cats}
    L = [f"runs={len(runs)} tasks={len(tasks)}"]
    L.append("\nPER-ARM ACCURACY (task-level, any-seed)")
    for a in ARMS:
        rs = by_arm[a]
        k = sum(1 for r in rs if r["correct"]); n = len(rs)
        lo, hi = wilson(k, n)
        report["arms"][a] = {"correct": k, "n": n, "acc": k / n, "ci": [lo, hi]}
        L.append(f"  {a:14s} {k:3d}/{n}  acc={k/n:.3f}  CI=[{lo:.3f},{hi:.3f}]")
    L.append("\nPER-SUITE ACCURACY")
    L.append("  " + "arm".ljust(14) + "".join(s.rjust(10) for s in SUITES))
    for a in ARMS:
        row = "  " + a.ljust(14)
        for s in SUITES:
            rs = [r for r in by_arm[a] if r["suite"] == s]
            k = sum(1 for r in rs if r["correct"])
            row += f"{k}/{len(rs)}".rjust(10)
        L.append(row)
    L.append("\nPER-CATEGORY ACCURACY")
    L.append("  " + "arm".ljust(14) + "".join(c.split("_")[0].rjust(6) for c in cats))
    for a in ARMS:
        row = "  " + a.ljust(14)
        for c in cats:
            rs = [r for r in by_arm[a] if r["category"] == c]
            k = sum(1 for r in rs if r["correct"])
            row += f"{k}/{len(rs)}".rjust(6)
        L.append(row)
    L.append("\nPAIRED (task-level; A/B/ties, exact McNemar p)")
    for a, b in [("D_intelligent", "B_original"), ("D_intelligent", "C_semantic"),
                 ("C_semantic", "B_original"), ("D_intelligent", "A_none"),
                 ("D_intelligent", "E_oracle"), ("B_original", "A_none")]:
        aw = bw = t = 0
        for tk in tasks:
            ca, cb = correct.get((tk, a)), correct.get((tk, b))
            if ca is None or cb is None:
                continue
            if ca and not cb: aw += 1
            elif cb and not ca: bw += 1
            else: t += 1
        pv = binom(aw, aw + bw)
        report["paired"][f"{a}_vs_{b}"] = {"a": aw, "b": bw, "ties": t, "p": pv}
        L.append(f"  {a:14s} vs {b:14s}: {aw} / {bw} / {t}  p={pv:.4f}")
    L.append("\nEVIDENCE RECALL (mean required-evidence recall@K)")
    for a in ARMS:
        m = sum(r["evidence_recall"] for r in by_arm[a]) / len(by_arm[a])
        report["arms"][a]["evidence_recall"] = m
        L.append(f"  {a:14s} {m:.3f}")
    L.append("\nLATENCY / TOKENS (mean per run)")
    for a in ARMS:
        rs = by_arm[a]
        L.append(f"  {a:14s} agent_s={sum(r['agent_seconds'] for r in rs)/len(rs):.2f} "
                 f"prompt_tok={sum(r['prompt_tokens'] for r in rs)/len(rs):.0f}")
    L.append("\nFAILURE TAXONOMY (D_intelligent failures by category)")
    fails = defaultdict(int)
    for r in by_arm["D_intelligent"]:
        if not r["correct"]:
            fails[r["category"]] += 1
    for c in sorted(fails):
        L.append(f"  {c:28s} {fails[c]}")
    L.append("\nSECURITY: forbidden-token leaks by arm (B5 + C3)")
    for a in ARMS:
        leaks = sum(1 for r in by_arm[a] if r["category"] in ("B5_trust_adversarial", "C3_security_ops") and r["forbidden_leaked"])
        tot = sum(1 for r in by_arm[a] if r["category"] in ("B5_trust_adversarial", "C3_security_ops"))
        L.append(f"  {a:14s} leaks={leaks}/{tot}")
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    Path(args.text).write_text("\n".join(L) + "\n", encoding="utf-8")
    print("\n".join(L))


if __name__ == "__main__":
    main()
