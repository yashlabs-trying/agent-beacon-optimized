# Phase 5 Raw Evidence

Produced on WSL2 with the Apple M5 Mac (`192.0.2.10`) as the remote Ollama +
BGE-small host. No downloads, no paid compute.

| File | Meaning |
|---|---|
| `selection_dump.json` | Per-task semantic candidates (arm C) and intelligent selection (arm D). |
| `results_suiteA.json`, `results_suiteB.json`, `results_suiteC.json` | 500 runs each (100 tasks × 5 arms). |
| `results_seeds12.json` | Seeds 1–2 on the 100-task discriminating subset (identical to seed 0). |
| `analysis.json/.txt` | Aggregate + paired statistics + failure taxonomy. |
| `calibration.json`, `calibration_c1.json` | Calibration runs (pre-freeze). |
| `selector_scaling_run.txt`, `selector_scaling.txt` | Indexed vs O(n²) selector. |
| `retrieval_scale.json` | Independent BM25/FTS5/recency at 1K/10K/100K. |
| `reliability.txt`, `race.txt` | Full learning suite + race subset. |

Traces: `../traces/` (one JSON per run). Fixtures: `../fixtures/`. Manifest: `../BENCHMARK_MANIFEST.json`.

Model `qwen2.5-coder:7b` (temp 0, seed 0/1/2, ctx 4096, predict 384).
