#!/usr/bin/env bash
# Phase 4D reproduction script: builds the optimized layer, runs contract/scaling tests,
# the CLI opt-in check, the dev + held-out pilots, and the reliability suites.
set -euo pipefail
REPO="${REPO:-/home/<user>/work/phase4d}"
BASELINE="${BASELINE:-/home/<user>/work/phase3a-baseline}"
PHASE4C="${PHASE4C:-/home/<user>/work/phase4c}"
MAC="${MAC:-192.0.2.10}"
OLLAMA_URL="http://${MAC}:11434/api/chat"; EMBED_URL="http://${MAC}:8077"
export PATH="/home/<user>/tools/go/bin:${PATH}"; export GOTOOLCHAIN=local
R="${REPO}/reports/phase4/part4/RAW_EVIDENCE"; BIN="${R}/bin"; mkdir -p "${BIN}"

python3 "${REPO}/reports/phase3/heldout/validate_lock.py"
echo PLACEHOLDER > "${REPO}/cli/beacon/internal/embedded/hooks.bin"
( cd "${REPO}/cli/beacon" && go build -o "${BIN}/beacon-v6" . )
( cd "${BASELINE}/cli/beacon" && go build -o "${BIN}/beacon-original" . )
( cd "${REPO}/cli/beacon" && go build -o "${BIN}/phase4cseed" ./internal/learning/testdata/phase4cseed )

( cd "${REPO}/cli/beacon" && go test ./internal/learning/ -run 'TestContract_|TestSelectContext_|TestPhase4CMemoryHistory' -v -count=1 ) | tee "${R}/contract_tests.txt"
( cd "${REPO}/cli/beacon" && BEACON_PHASE4D_SCALING_OUT="${R}/selector_scaling.txt" go test ./internal/learning/ -run TestPhase4DSelectorScaling -v -count=1 ) | tee "${R}/selector_scaling_run.txt"
( cd "${REPO}/cli/beacon" && go test ./internal/learning/ -run '^$' -bench BenchmarkSelectTrustContext1000 -benchmem -count=1 ) | tee "${R}/selector_bench.txt"

( cd "${REPO}/cli/beacon" && BEACON_PHASE4C_SELECT=1 BEACON_PHASE4C_FIXTURE="${PHASE4C}/reports/phase4/part3/fixtures/phase4c_dev_v1.json" \
    BEACON_PHASE4C_OUT="${R}/selection_dump.json" BEACON_PHASE4C_K=8 BEACON_EMBED_URL="${EMBED_URL}" \
    go test ./internal/learning/ -run TestPhase4CSelect -count=1 ) | tee "${R}/selection_dump.txt"

rm -rf /tmp/p4d-runs
python3 "${REPO}/reports/phase4/part3/harness/agent_pilot.py" \
  --fixture "${PHASE4C}/reports/phase4/part3/fixtures/phase4c_dev_v1.json" \
  --selection "${R}/selection_dump.json" --run-root /tmp/p4d-runs --bin-dir "${BIN}" --seed "${BIN}/phase4cseed" \
  --ollama-url "${OLLAMA_URL}" --model qwen2.5-coder:7b \
  --arms none,original,v6_semantic,v6_intelligent,oracle --output "${R}/agent_pilot.json" | tee "${R}/agent_pilot.txt"
python3 "${REPO}/reports/phase4/part3/harness/analyze_pilot.py" --runs "${R}/agent_pilot.json" --output "${R}/analysis.json" --text "${R}/analysis.txt"

( cd "${REPO}/cli/beacon" && go test ./internal/learning/ -count=1 ) | tee "${R}/full_learning.txt"
( cd "${REPO}/cli/beacon" && go test -race ./internal/learning/ -run 'Concurrency|Concurrent|PutMemoryWaits' -count=1 ) | tee "${R}/race.txt"
python3 "${REPO}/reports/phase3/heldout/validate_lock.py"
echo DONE
