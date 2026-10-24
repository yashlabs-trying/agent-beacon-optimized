#!/usr/bin/env python3
"""Estimate common cold-process CLI overhead with a no-database command."""
import json
import random
import statistics
import subprocess
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent
OUT = ROOT / 'startup_cli.json'
if OUT.exists():
    raise SystemExit('refusing overwrite')
bins = {
    'original': Path('/home/<user>/work/phase5/reports/phase5/bin/beacon-original'),
    'v7': Path('/home/<user>/work/beacon-v7-hardening-profile/cli/beacon/beacon-v7-audit'),
}
arms = list(bins) * 56
random.Random(20261023).shuffle(arms)
rows = []
for arm in arms:
    t0 = time.perf_counter_ns()
    proc = subprocess.run([str(bins[arm]), 'version'], capture_output=True, timeout=60)
    elapsed = time.perf_counter_ns() - t0
    if proc.returncode:
        raise SystemExit(f'{arm} version failed: {proc.stderr!r}')
    rows.append({'arm': arm, 'elapsed_ns': elapsed})
OUT.write_text(json.dumps({'rows': rows}, indent=2) + '\n')
for arm in bins:
    values = [r['elapsed_ns']/1e6 for r in rows if r['arm']==arm]
    print(arm, f'mean={statistics.mean(values):.3f}ms median={statistics.median(values):.3f}ms')
