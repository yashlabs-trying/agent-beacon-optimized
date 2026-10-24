# Phase 4C Raw Evidence

Produced by `reports/phase4/part3/run_phase4c.sh` on WSL2 with the Apple M5 Mac as the
remote Ollama + BGE-small host. No model downloads, no paid compute.

| File | Meaning |
|---|---|
| `corpus_build.txt`, `dev_manifest.json`, `heldout_manifest.json` | Corpus build + hashes. |
| `selection_dump.txt/.json` | Dev: raw retrieval + SelectTrustContext output per ablation config. |
| `selection_dump_heldout.txt/.json` | Held-out selection dump. |
| `agent_pilot.txt/.json` | Dev: 315 runs (35 cases × 9 arms). |
| `agent_pilot_heldout.txt/.json` | Held-out: 90 runs (18 cases × 5 arms). |
| `analysis.txt/.json`, `analysis_heldout.txt/.json` | Per-arm/category accuracy, paired McNemar, ablations. |
| `unit_tests.txt`, `memory_history.txt` | Layer unit tests + 10/50/100-session history test. |
| `reliability_full.txt`, `reliability_race.txt` | Regression suites. |
| `bin/` | Arm binaries (git-ignored; rebuild via `run_phase4c.sh`). |

Fixtures: dev `511B4C30…`, held-out `6461AF90…` (SHA-256, CRLF→LF).
Model `qwen2.5-coder:7b` (temp 0, seed 42, ctx 4096, predict 256); BGE-small dim=384 (MPS).
