# Phase 4B Raw Evidence

Produced by `reports/phase4/part2/run_phase4b.sh` on WSL2 with the Apple M5 Mac as the
remote Ollama + BGE-small host. No model downloads, no paid compute.

| File | Meaning |
|---|---|
| `corpus_build.txt` | Dev corpus build + validation output. |
| `retrieval_dump.txt/.json` | In-process top-5 IDs per case for lexical/hashed/hybrid/neural. |
| `agent_pilot.txt/.json` | 140 runs (20 cases × 7 arms): answer, grade, retrieved IDs, recall, latency, tokens. |
| `agent_pilot_lexical.json` | Re-run of the two CLI lexical arms with correct binaries (merged into `agent_pilot.json`). |
| `analysis.txt/.json` | Per-arm/category accuracy, paired McNemar, recall, latency. |
| `bin/` | Arm binaries (git-ignored; rebuild via `run_phase4b.sh`). |

Environment: WSL2 `Linux 6.18.33.2-microsoft-standard-WSL2`, 8 vCPU, 7.6 GiB, Go 1.27.3;
Mac `macOS 26.6.2`, Apple M5, 16 GiB, Ollama 0.40.6 (qwen2.5-coder:7b),
BGE-small dim=384 (MPS).
