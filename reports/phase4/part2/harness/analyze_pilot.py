#!/usr/bin/env python3
"""Phase 4B Stage C/E — paired analysis of the adversarial agent pilot.

Reads the agent_pilot.json runs and reports per-arm/per-category accuracy, paired
win/loss/tie counts with exact (McNemar) binomial p-values, required-evidence recall,
and latency/token summaries. Logical tasks are the independent unit.
"""
import argparse
import json
import math
from collections import defaultdict
from pathlib import Path

ARMS = ["none", "original", "v6_lexical", "v6_hashed", "v6_hybrid", "v6_neural", "oracle"]


def binom_two_sided(k, n, p=0.5):
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
    by_arm = defaultdict(list)
    for r in runs:
        by_arm[r["arm"]].append(r)
    cases = sorted({r["case"] for r in runs})
    categories = sorted({r["category"] for r in runs})

    # correctness lookup
    correct = {(r["case"], r["arm"]): r["correct"] for r in runs}
    report = {"n_cases": len(cases), "arms": {}, "categories": categories,
              "paired": {}, "latency": {}, "evidence_recall": {}}

    lines = []
    lines.append(f"cases={len(cases)} arms={len(ARMS)} runs={len(runs)}")
    lines.append("")
    lines.append("PER-ARM ACCURACY (correct/total, Wilson 95% CI)")
    for a in ARMS:
        rs = by_arm[a]
        k = sum(1 for r in rs if r["correct"])
        n = len(rs)
        lo, hi = wilson(k, n)
        report["arms"][a] = {"correct": k, "n": n, "acc": k / n, "ci": [lo, hi]}
        lines.append(f"  {a:12s} {k:2d}/{n}  acc={k/n:.3f}  CI=[{lo:.3f},{hi:.3f}]")

    lines.append("")
    lines.append("PER-CATEGORY ACCURACY")
    header = "  " + "arm".ljust(12) + "".join(c[:8].rjust(9) for c in categories)
    lines.append(header)
    for a in ARMS:
        row = "  " + a.ljust(12)
        for c in categories:
            rs = [r for r in by_arm[a] if r["category"] == c]
            k = sum(1 for r in rs if r["correct"])
            row += f"{k}/{len(rs)}".rjust(9)
        lines.append(row)

    lines.append("")
    lines.append("PAIRED COMPARISONS (A vs B: A-wins / B-wins / ties, exact McNemar p)")
    pairs = [("v6_neural", "original"), ("v6_neural", "v6_hashed"),
             ("v6_neural", "none"), ("v6_hashed", "original"),
             ("oracle", "v6_neural"), ("v6_lexical", "original")]
    for a, b in pairs:
        aw = bw = ties = 0
        for c in cases:
            ca, cb = correct.get((c, a)), correct.get((c, b))
            if ca is None or cb is None:
                continue
            if ca and not cb:
                aw += 1
            elif cb and not ca:
                bw += 1
            else:
                ties += 1
        disc = aw + bw
        pval = binom_two_sided(aw, disc)
        report["paired"][f"{a}_vs_{b}"] = {"a_wins": aw, "b_wins": bw, "ties": ties, "p": pval}
        lines.append(f"  {a:12s} vs {b:12s}: {aw} / {bw} / {ties}  p={pval:.4f}")

    lines.append("")
    lines.append("REQUIRED-EVIDENCE RECALL@K (mean over cases)")
    for a in ARMS:
        rs = by_arm[a]
        m = sum(r["required_evidence_recall"] for r in rs) / len(rs)
        report["evidence_recall"][a] = m
        lines.append(f"  {a:12s} {m:.3f}")

    lines.append("")
    lines.append("LATENCY / TOKENS (mean over cases)")
    for a in ARMS:
        rs = by_arm[a]
        lat = sum(r["agent_seconds"] for r in rs) / len(rs)
        pt = sum(r["prompt_tokens"] for r in rs) / len(rs)
        gt = sum(r["gen_tokens"] for r in rs) / len(rs)
        report["latency"][a] = {"agent_s": lat, "prompt_tokens": pt, "gen_tokens": gt}
        lines.append(f"  {a:12s} agent_s={lat:.2f} prompt_tok={pt:.0f} gen_tok={gt:.0f}")

    Path(args.output).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    Path(args.text).write_text("\n".join(lines) + "\n", encoding="utf-8")
    print("\n".join(lines))


if __name__ == "__main__":
    main()
