#!/usr/bin/env python3
"""Audit-only paired cold-process retrieval timing; no model calls."""
import hashlib
import json
import random
import subprocess
import time
from pathlib import Path

ROOT = Path('/home/<user>/work/beacon-v7-hardening-profile')
EVIDENCE = Path('/mnt/c/Users/yashs/work/beacon-v7-hardening/reports/v7-hardening/raw/profile')
FIXTURE = ROOT / 'reports/v7/validation/holdout_v1.json'
SEED = Path('/home/<user>/work/phase5/reports/phase5/bin/phase5seed')
ORIGINAL = Path('/home/<user>/work/phase5/reports/phase5/bin/beacon-original')
V7 = Path('/home/<user>/work/beacon-v7-hardening-profile/cli/beacon/beacon-v7-audit')
WORK = Path('/home/<user>/work/v7-phase-a-cold-20261023')
OUT = EVIDENCE / 'cold_cli.json'

if WORK.exists() or OUT.exists():
    raise SystemExit('refusing to overwrite existing work or evidence')
WORK.mkdir(parents=True)
fixture_bytes = FIXTURE.read_bytes()
tasks = json.loads(fixture_bytes)['tasks']
rng = random.Random(20261023)
rng.shuffle(tasks)

def run(arm, task, workspace, pass_name):
    binary = ORIGINAL if arm == 'original' else V7
    cmd = [str(binary), 'memory', 'list', '--json', '--limit', '8',
           '--query', task['retrieval_query'], '--project', str(workspace),
           '--log-path', str(workspace / 'events.jsonl')]
    if arm == 'v7':
        cmd += ['--question-aware', '--question', task['request'], '--semantic']
    t0 = time.perf_counter_ns()
    proc = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    elapsed = time.perf_counter_ns() - t0
    if proc.returncode:
        raise RuntimeError(f'{task["id"]} {arm}: {proc.stderr}')
    result = json.loads(proc.stdout)
    return {'task': task['id'], 'category': task['category'], 'arm': arm,
            'pass': pass_name, 'elapsed_ns': elapsed,
            'route': result.get('route') if arm == 'v7' else None}

rows = []
for task in tasks:
    workspace = WORK / task['id']
    workspace.mkdir()
    subprocess.run([str(SEED), task['id'], str(workspace), str(workspace / 'memory.db'), str(FIXTURE)],
                   check=True, capture_output=True, text=True, timeout=60)
    # Preserve original-before-backfill order of the previous validation.
    rows.append(run('original', task, workspace, 'first'))
    rows.append(run('v7', task, workspace, 'first'))
    arms = ['original', 'v7']
    rng.shuffle(arms)
    for arm in arms:
        rows.append(run(arm, task, workspace, 'repeat'))

OUT.write_text(json.dumps({'source_commit': '7b605aa84f0f570ba265fbda17fe6d99e927b10e',
                           'fixture_sha256': hashlib.sha256(fixture_bytes).hexdigest(),
                           'original_sha256': hashlib.sha256(ORIGINAL.read_bytes()).hexdigest(),
                           'v7_sha256': hashlib.sha256(V7.read_bytes()).hexdigest(),
                           'rows': rows}, indent=2) + '\n')
print(f'wrote {OUT} ({len(rows)} rows)')
