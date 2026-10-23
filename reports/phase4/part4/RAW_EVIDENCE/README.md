# Phase 4D Raw Evidence

Produced by `reports/phase4/part4/run_phase4d.sh` on WSL2 with the Apple M5 Mac as the
remote Ollama + BGE-small host. No downloads, no paid compute.

| File | Meaning |
|---|---|
| `contract_tests.txt` | 12 cross-component interaction tests. |
| `selector_scaling.txt` / `selector_scaling_run.txt` | Indexed vs O(n²) at 20/100/500/1000. |
| `selector_bench.txt` | BenchmarkSelectTrustContext1000. |
| `selection_dump.json/.txt` | Optimized-layer selection over the dev corpus. |
| `agent_pilot.json/.txt`, `analysis.json/.txt` | Dev pilot (175 runs). |
| `agent_pilot_heldout.json/.txt`, `analysis_heldout.*` | Held-out pilot (90 runs). |
| `full_learning.txt`, `race.txt` | Regression suites. |
| `gofmt.txt`, `vet.txt`, `build.txt` | Static checks. |
| `bin/` | Arm binaries (git-ignored; rebuild via `run_phase4d.sh`). |

Model `qwen2.5-coder:7b` (temp 0, seed 42, ctx 4096, predict 256). Fixtures unchanged from
Phase 4C (dev `511B4C30…`, held-out `6461AF90…`); Phase 3 locks re-verified.
