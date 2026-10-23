# Phase 4D — End-to-End Development Results (Stage G)

Re-run of the Phase 4C development pilot with the **optimized** Phase 4D layer, same
model and frozen fixture, plus the CLI opt-in path.

- Fixture: Phase 4C dev `phase4c_dev_v1.json` (35 cases, `511B4C30…`).
- Model: qwen2.5-coder:7b (temp 0, seed 42, ctx 4096, predict 256).
- Arms: none, original, v6_semantic, v6_intelligent, oracle (175 runs).
- Evidence: `RAW_EVIDENCE/agent_pilot.json`, `analysis.txt`.

## 1. Results

| Arm | Correct | Accuracy | 95% CI |
|---|---:|---:|---|
| none | 6/35 | 0.171 | [0.081, 0.327] |
| original | 6/35 | 0.171 | [0.081, 0.327] |
| v6_semantic | 20/35 | 0.571 | [0.409, 0.720] |
| **v6_intelligent** | **27/35** | **0.771** | [0.610, 0.879] |
| oracle | 27/35 | 0.771 | [0.610, 0.879] |

Paired: v6_intelligent vs v6_semantic **7–0–28, p=0.0156**; vs original **21–0–14,
p<1e-4**; vs oracle **0–0–35, p=1.0**.

Category: contradiction 7/11, ambiguity **8/8**, adversarial 6/6, decision 6/6, control
0/4 (invalid controls — see below).

## 2. Before/after vs Phase 4C

| Arm | Phase 4C | Phase 4D |
|---|---:|---:|
| v6_intelligent | 26/35 | **27/35** |
| ambiguity | 7/8 | **8/8** |
| v6_intelligent vs semantic | 6–0 (p=0.031) | **7–0 (p=0.0156)** |

No regression; the sufficiency fix removed the last ambiguity failure and the intelligent
arm now matches the oracle exactly.

## 3. Held-out (no regression)

| Arm | Phase 4C | Phase 4D |
|---|---:|---:|
| v6_semantic | 10/18 | 10/18 |
| v6_intelligent | 14/18 | 14/18 |
| oracle | 15/18 | 15/18 |

v6_intelligent vs v6_semantic held-out: 4–0 (p=0.125) — unchanged, still directional.

## 4. CLI end-to-end opt-in (new)

Verified against a seeded DB:

```
beacon memory list --json --semantic --intelligent --query "API timeout seconds" ...
→ kept ["to2-verified"]; signals.untrusted_ids ["to2-fake"]
```

Defaults are unchanged without the flags (`--semantic`/`--intelligent`/`--sufficiency`
default false).

## 5. Controls (known-invalid)

The note-only prompt makes all arms (including oracle) abstain on memory-independent
controls (0/4 dev, 0/2 held-out). This is a **harness-prompt defect**, excluded from the
memory comparison; the fix (a prompt permitting general knowledge for no-memory tasks) is
specified for Phase 5.

## Verdict

End-to-end dev: **PASS** — no regression, improved accuracy, oracle parity, reachable via
the CLI. Controls: **CONDITIONAL** (prompt fix pending).
