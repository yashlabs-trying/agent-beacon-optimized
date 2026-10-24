#!/usr/bin/env python3
"""Recompute paired CLI latency summaries from immutable raw rows."""
import json
import statistics
from collections import Counter, defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent
rows = json.loads((ROOT / 'cold_cli.json').read_text())['rows']

def pct(values, percentile):
    values = sorted(values)
    return values[min(len(values)-1, (len(values)*percentile + 99)//100 - 1)]

lines = [f'rows={len(rows)} tasks={len(set(r["task"] for r in rows))}']
for pass_name in ('first', 'repeat'):
    for arm in ('original', 'v7'):
        values = [r['elapsed_ns']/1e6 for r in rows if r['pass'] == pass_name and r['arm'] == arm]
        lines.append(f'{pass_name} {arm}: n={len(values)} mean={statistics.mean(values):.3f}ms '
                     f'p50={statistics.median(values):.3f}ms p95={pct(values,95):.3f}ms '
                     f'p99={pct(values,99):.3f}ms')
    paired = defaultdict(dict)
    for r in rows:
        if r['pass'] == pass_name:
            paired[r['task']][r['arm']] = r['elapsed_ns']/1e6
    differences = [v['v7']-v['original'] for v in paired.values()]
    lines.append(f'{pass_name} paired V7-original: median={statistics.median(differences):.3f}ms '
                 f'mean={statistics.mean(differences):.3f}ms slower={sum(v>0 for v in differences)}/{len(differences)}')
lines.append(f'V7 routes: {dict(Counter((r["pass"],r["route"]) for r in rows if r["arm"]=="v7"))}')
for pass_name in ('first', 'repeat'):
    for route in ('lexical', 'semantic'):
        values = [r['elapsed_ns']/1e6 for r in rows if r['pass']==pass_name and r['arm']=='v7' and r['route']==route]
        lines.append(f'{pass_name} V7 {route}: n={len(values)} mean={statistics.mean(values):.3f}ms '
                     f'p50={statistics.median(values):.3f}ms p95={pct(values,95):.3f}ms')
out = ROOT / 'cold_cli_summary_v2.txt'
if out.exists():
    raise SystemExit('refusing overwrite')
out.write_text('\n'.join(lines)+'\n')
print(out.read_text())
