#!/usr/bin/env python3
"""Validate relative markdown links in the files this change added/modified."""
import os, re, sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
FILES = [
    "README.md", "CONTRIBUTING.md", "CHANGELOG.md", "CODE_OF_CONDUCT.md", "SECURITY.md",
    "examples/README.md",
    "examples/memory-basic/README.md", "examples/memory-scale/README.md",
    "examples/agent-mcp/README.md",
    "docs/optimization/index.md", "docs/optimization/architecture.md",
    "docs/optimization/performance.md", "docs/optimization/benchmarks.md",
    "docs/optimization/security.md", "docs/optimization/configuration.md",
    "docs/optimization/limitations.md", "docs/optimization/roadmap.md",
    "docs/optimization/retrieval-pipeline.md", "docs/optimization/troubleshooting.md",
]
LINK = re.compile(r"\[[^\]]+\]\(([^)]+)\)")
SKIP_PREFIX = ("http://", "https://", "#", "mailto:")

bad = 0
checked = 0
for rel in FILES:
    path = os.path.join(ROOT, rel)
    if not os.path.exists(path):
        print(f"MISSING FILE: {rel}")
        bad += 1
        continue
    text = open(path, encoding="utf-8").read()
    for m in LINK.finditer(text):
        target = m.group(1).strip()
        if target.startswith(SKIP_PREFIX):
            continue
        # strip anchor
        target_path = target.split("#", 1)[0]
        if not target_path:
            continue
        resolved = os.path.normpath(os.path.join(os.path.dirname(path), target_path))
        checked += 1
        if not os.path.exists(resolved):
            print(f"BROKEN in {rel}: {target}")
            bad += 1

# anchor check within docs
print(f"checked {len(FILES)} files, {checked} relative links")
if bad:
    print(f"RESULT: FAIL ({bad} problems)")
    sys.exit(1)
print("RESULT: PASS — all relative links resolve")
