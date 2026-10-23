#!/usr/bin/env python3
"""Phase 4C paired analysis: per-arm/per-category accuracy, paired McNemar,
ablation deltas, evidence recall, and latency. Logical tasks are the unit."""
import argparse
import json
import math
from collections import defaultdict
from pathlib import Path

ARMS = ["none", "original", "v6_semantic", "v6_intelligent", "oracle",
        "ab_supersession", "ab_trust", "ab_conflict", "ab_injection"]
CATS = ["contradiction", "ambiguity", "adversarial", "decision", "control"]


def binom(k, n, p=0.5):
    if n == 0:
        return 1.0
    probs = [math.comb(n, i) * p ** i * (1 - p) ** (n - i) for i in range(n + 1)]
    obs = probs[k]
    return min(1.0, sum(pr for pr in probs if pr <= obs + 1e-12))


def wilson(k, n, z=1.96):
    if n == 0:
        return (0.0, 0.0)
    p = k / n
    d = 1 + z * z / n
    c = p + z * z / (2 * n)
    m = z * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n))
    return ((c - m) / d, (c + m) / d)


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--runs", required=True)
    p.add_argument("--output", required=True)
    p.add_argument("--text", required=True)
    args = p.parse_args()
    runs = json.loads(Path(args.runs).read_text(encoding="utf-8"))
    present = {r["arm"] for r in runs}
    ARMS_LOCAL = [a for a in ARMS if a in present]
    by_arm = defaultdict(list)
    for r in runs:
        by_arm[r["arm"]].append(r)
    cases = sorted({r["case"] for r in runs})
    correct = {(r["case"], r["arm"]): r["correct"] for r in runs}
    report = {"arms": {}, "paired": {}, "categories": CATS}
    L = []
    L.append(f"cases={len(cases)} arms={len(ARMS_LOCAL)} runs={len(runs)}")
    L.append("")
    L.append("PER-ARM ACCURACY (Wilson 95% CI)")
    for a in ARMS_LOCAL:
        rs = by_arm[a]
        k, n = sum(1 for r in rs if r["correct"]), len(rs)
        lo, hi = wilson(k, n)
        report["arms"][a] = {"correct": k, "n": n, "acc": k / n, "ci": [lo, hi]}
        L.append(f"  {a:16s} {k:2d}/{n}  acc={k/n:.3f}  CI=[{lo:.3f},{hi:.3f}]")
    L.append("")
    L.append("PER-CATEGORY ACCURACY")
    L.append("  " + "arm".ljust(16) + "".join(c[:8].rjust(9) for c in CATS))
    for a in ARMS_LOCAL:
        row = "  " + a.ljust(16)
        for c in CATS:
            rs = [r for r in by_arm[a] if r["category"] == c]
            k = sum(1 for r in rs if r["correct"])
            row += f"{k}/{len(rs)}".rjust(9)
        L.append(row)
    L.append("")
    L.append("PAIRED COMPARISONS (A vs B: A / B / ties, exact McNemar p)")
    pairs = [("v6_intelligent", "v6_semantic"), ("v6_intelligent", "original"),
             ("v6_intelligent", "none"), ("v6_intelligent", "oracle"),
             ("v6_semantic", "original")]
    for a, b in pairs:
        aw = bw = t = 0
        for c in cases:
            ca, cb = correct.get((c, a)), correct.get((c, b))
            if ca is None or cb is None:
                continue
            if ca and not cb:
                aw += 1
            elif cb and not ca:
                bw += 1
            else:
                t += 1
        report["paired"][f"{a}_vs_{b}"] = {"a": aw, "b": bw, "ties": t, "p": binom(aw, aw + bw)}
        L.append(f"  {a:16s} vs {b:16s}: {aw} / {bw} / {t}  p={binom(aw, aw+bw):.4f}")
    L.append("")
    L.append("ABLATION DELTAS vs v6_semantic (paired)")
    base = "v6_semantic"
    for a in ["ab_supersession", "ab_trust", "ab_conflict", "ab_injection", "v6_intelligent"]:
        aw = bw = t = 0
        for c in cases:
            ca, cb = correct.get((c, a)), correct.get((c, base))
            if ca is None or cb is None:
                continue
            if ca and not cb:
                aw += 1
            elif cb and not ca:
                bw += 1
            else:
                t += 1
        L.append(f"  {a:16s} vs {base:12s}: +{aw} / -{bw} / ={t}  p={binom(aw, aw+bw):.4f}")
    L.append("")
    L.append("MEAN REQUIRED-EVIDENCE RECALL")
    for a in ARMS_LOCAL:
        m = sum(r["evidence_recall"] for r in by_arm[a]) / len(by_arm[a])
        report["arms"][a]["evidence_recall"] = m
        L.append(f"  {a:16s} {m:.3f}")
    L.append("")
    L.append("MEAN LATENCY / TOKENS")
    for a in ARMS_LOCAL:
        rs = by_arm[a]
        L.append(f"  {a:16s} agent_s={sum(r['agent_seconds'] for r in rs)/len(rs):.2f} "
                 f"prompt_tok={sum(r['prompt_tokens'] for r in rs)/len(rs):.0f}")
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    Path(args.text).write_text("\n".join(L) + "\n", encoding="utf-8")
    print("\n".join(L))


if __name__ == "__main__":
    main()
