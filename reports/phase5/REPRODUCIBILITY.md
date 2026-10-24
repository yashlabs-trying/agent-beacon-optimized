# Phase 5 — Reproducibility

## 1. Provenance

| Item | Value |
|---|---|
| Original Beacon | `99c709081ad49da1c5ade205929b61b19360ed9b` |
| Phase 4D (frozen comparison) | `7c7e24933dbef69fe47599c35c211ee24f383a1d` |
| Phase 5 branch | `phase5/beacon-ultimate-eval` (worktree `/home/<user>/work/phase5`) |
| Phase 3 locked fixtures | `LOCK OK ×3` (unchanged) |

Ancestry `99c70908 → bcf3a1fd → e032ee73 → 2932e8e5 → 12226058 → 7c7e2493` verified.

## 2. Benchmark

- Fixture: `reports/phase5/fixtures/phase5_heldout_v1.json`
- SHA-256: `98307FED1FFF199F3447319428C8802C03C10511898EE4FCA553D8EF55C8DBE7`
- Generator seed: 20261023 (reproduces the identical file).
- Manifest: `reports/phase5/BENCHMARK_MANIFEST.json`.

## 3. Environment

| Host | Spec |
|---|---|
| WSL2 laptop | Linux 6.18.33.2-microsoft-standard-WSL2, 8 vCPU, 7.6 GiB, Go 1.27.3, Python 3.14.4 |
| Apple M5 Mac | macOS 26.6.2, 16 GiB, Ollama 0.40.6, BGE-small dim=384 (MPS) |

Model `qwen2.5-coder:7b`; temperature 0; seeds 0/1/2; num_ctx 4096; num_predict 384.
Dependencies: Go 1.27.3, Python 3.14.4 stdlib, `sentence-transformers` on the Mac.

## 4. Exact commands (all run from `/home/<user>/work/phase5`)

```sh
# generate + freeze the 300-task suite
python3 reports/phase5/harness/generate_tasks.py \
  --out reports/phase5/fixtures/phase5_heldout_v1.json \
  --manifest reports/phase5/BENCHMARK_MANIFEST.json

# build binaries
( cd cli/beacon && go build -o reports/phase5/bin/beacon-v6 . )
( cd cli/beacon && go build -o reports/phase5/bin/phase5seed ./internal/learning/testdata/phase5seed )
( cd /home/<user>/work/phase3a-baseline/cli/beacon && go build -o <abs>/reports/phase5/bin/beacon-original . )

# selection dump (arms C/D candidates)
( cd cli/beacon && BEACON_PHASE5_SELECT=1 BEACON_PHASE5_FIXTURE=<abs>/reports/phase5/fixtures/phase5_heldout_v1.json \
    BEACON_PHASE5_OUT=<abs>/reports/phase5/raw/selection_dump.json BEACON_PHASE5_K=8 \
    BEACON_EMBED_URL=http://192.0.2.10:8077 go test ./internal/learning/ -run TestPhase5Select -count=1 )

# five-arm comparison (per suite)
python3 reports/phase5/harness/agent_eval.py --fixture <fix> --selection <sel> \
  --run-root /tmp/p5 --bin-dir reports/phase5/bin --seed-bin reports/phase5/bin/phase5seed \
  --ollama-url http://192.0.2.10:11434/api/chat --model qwen2.5-coder:7b \
  --categories <suite cats> --trace-dir reports/phase5/traces --output reports/phase5/raw/results_suiteX.json

# analysis
python3 reports/phase5/harness/analyze_phase5.py --inputs raw/results_suite{A,B,C}.json \
  --output raw/analysis.json --text raw/analysis.txt

# performance / reliability
( cd cli/beacon && go test ./internal/learning/ -run TestPhase4DSelectorScaling -v -count=1 )
( cd cli/beacon && go test ./internal/learning/ -count=1 )          # ok 47.6 s
( cd cli/beacon && go test -race ./internal/learning/ -run 'Concurrency|Crash' -count=1 )  # ok 34.5 s
```

Exit codes: all test/build commands exited 0.

## 5. Raw evidence

`reports/phase5/raw/` (per-suite results, analysis, selection dump, scaling, reliability),
`reports/phase5/traces/` (per-run JSON), `reports/phase5/fixtures/`.

## 6. Gaps

1M memories, 32/64 clients, neural-in-the-agent-loop, external benchmarks (LongMemEval /
LoCoMo / MemoryAgentBench), and sandboxed tool-execution security are **NOT TESTED**.
