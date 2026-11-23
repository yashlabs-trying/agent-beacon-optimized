# Phase 4A Raw Evidence Manifest

All files produced by `reports/phase4/part1/run_phase4a.sh` on the WSL2 laptop
(Go 1.27.3, Python 3.14.4) using the Apple M5 Mac (`192.0.2.10`) as the
remote Ollama + BGE-small host. No paid compute, no model downloads.

| File | Produced by | Meaning |
|---|---|---|
| `retrieval_neural_mac.txt` | `phase3a_retrieval_test.go` (`TestPhase3ARetrieval*`) | Locked 48-query retrieval: lexical, hashed, hybrid, **real neural BGE**. |
| `retrieval_baselines.txt/.json` | `harness/retrieval_baselines.py` | Independent BM25 / FTS5 / recency / BGE-exact on the locked set + synthetic 1K/10K/100K. |
| `harness_validation.txt/.json` | `harness/validate_harness.py` | 16 harness checks (8 conditions + properties + negative control). |
| `validation_reference.json` | `agent_eval.py --script reference` | Scripted reference: 6/6 all arms (validation, not model). |
| `validation_noop.json` | `agent_eval.py --script noop` | Scripted noop: 0/6 all arms (validation, not model). |
| `model_capability_qwen2.5-coder-7b.json` | `harness/model_capability_check.py` | 7b capability check (SUITABLE). |
| `model_capability_qwen2.5-coder-1.5b.json` | `harness/model_capability_check.py` | 1.5b capability check (UNSUITABLE). |
| `six_task_pilot.txt/.json` | `agent_eval.py --backend ollama` | Real six-task three-arm pilot (none 4/6, original 6/6, v6 6/6). |
| `reliability_full_learning.txt` | `go test ./internal/learning/` | Full package suite. |
| `reliability_targeted.txt` | `go test -v -run 'Phase2\|Phase3\|Adversarial\|…'` | Reliability/security subset. |
| `reliability_race.txt` | `go test -race …` | Race subset. |
| `reliability_hnsw.txt` | `go test ./internal/hnsw/` | HNSW package. |
| `contradiction_locked_phase4a.txt` | `BEACON_PHASE3_CONTRADICTION=1 go test …` | Locked contradiction detail (precision 1.000, recall 0.500). |
| `contradiction_phase4a.txt` | `go test -v -run TestPhase3BContradictionDev` | Dev contradiction (default 0.769, cross-kind 1.000). |
| `bin/` | `go build` | Arm binaries (`beacon-v6`, `beacon-original`, `phase4aseed`). Git-ignored; rebuild via `run_phase4a.sh`. |

Environment: WSL2 `Linux 6.18.33.2-microsoft-standard-WSL2 x86_64`, 8 vCPU,
7.6 GiB; Mac `macOS 26.6.2`, Apple M5, 16 GiB, Ollama 0.40.6, BGE-small dim=384 (MPS).
