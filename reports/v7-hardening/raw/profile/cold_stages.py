#!/usr/bin/env python3
"""Time V7 internal stages on seven fresh original-style fixture stores."""
import hashlib
import json
import os
import re
import subprocess
from pathlib import Path

ROOT = Path('/home/<user>/work/beacon-v7-hardening-profile')
EVIDENCE = Path('/mnt/c/Users/yashs/work/beacon-v7-hardening/reports/v7-hardening/raw/profile')
FIXTURE = ROOT / 'reports/v7/validation/holdout_v1.json'
SEED = Path('/home/<user>/work/phase5/reports/phase5/bin/phase5seed')
WORK = Path('/home/<user>/work/v7-phase-a-stages-20261023')
OUT = EVIDENCE / 'cold_stages.json'
SRC = ROOT / 'cli/beacon/internal/learning/phase_a_stages_test.go'
if WORK.exists() or OUT.exists():
    raise SystemExit('refusing to overwrite work or evidence')
WORK.mkdir(parents=True)
SRC.write_bytes((EVIDENCE / SRC.name).read_bytes())
tasks = json.loads(FIXTURE.read_bytes())['tasks']
by_category = {}
for task in tasks:
    by_category.setdefault(task['category'], task)
rows = []
for category, task in sorted(by_category.items()):
    workspace = WORK / task['id']
    workspace.mkdir()
    db = workspace / 'memory.db'
    subprocess.run([str(SEED), task['id'], str(workspace), str(db), str(FIXTURE)],
                   check=True, capture_output=True, timeout=60)
    env = os.environ.copy()
    env.update(BEACON_AUDIT_DB=str(db), BEACON_AUDIT_PROJECT_PATH=str(workspace),
               BEACON_AUDIT_QUERY=task['retrieval_query'], BEACON_AUDIT_QUESTION=task['request'],
               BEACON_AUDIT_SEMANTIC='1' if category=='chain' else '0')
    proc = subprocess.run(['/home/<user>/sdk/go/bin/go', 'test', './internal/learning',
                           '-run', '^TestProfileV7Stages$', '-v', '-count=1'],
                          cwd=ROOT / 'cli/beacon', env=env, capture_output=True,
                          text=True, timeout=90)
    if proc.returncode:
        raise RuntimeError(f'{category}: {proc.stdout}\n{proc.stderr}')
    stages = {name: int(ns) for name, ns in re.findall(r'STAGE (\w+) (\d+)', proc.stdout)}
    route = re.search(r'ROUTE (\w+) (\d+) (\d+)', proc.stdout)
    if not stages or not route:
        raise RuntimeError(f'no stage logs: {proc.stdout}')
    rows.append({'task':task['id'],'category':category,'stages_ns':stages,
                 'route':route.group(1),'lexical_candidates':int(route.group(2)),
                 'semantic_candidates':int(route.group(3)),'go_test_stdout':proc.stdout})
OUT.write_text(json.dumps({'source_commit':'7b605aa84f0f570ba265fbda17fe6d99e927b10e',
                           'fixture_sha256':hashlib.sha256(FIXTURE.read_bytes()).hexdigest(),
                           'rows':rows}, indent=2)+'\n')
for row in rows:
    print(row['category'], row['route'], ' '.join(f'{k}={v/1e6:.3f}ms' for k,v in row['stages_ns'].items()))
