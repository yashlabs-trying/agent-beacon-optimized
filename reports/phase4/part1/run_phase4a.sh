#!/usr/bin/env bash
# Phase 4A reproduction script.
#
# Runs every feasible Phase 4A check on the WSL2 laptop, using the Apple M5 Mac
# as a remote model/embedding host. Does not download models, use paid compute,
# or push. Fixtures are locked and must not be edited.
#
# Prerequisites (already provisioned, not downloaded here):
#   * Mac reachable over SSH with Ollama (qwen2.5-coder:7b) and the BGE-small
#     sentence-transformers sidecar.
#   * Go 1.27.3 at /home/<user>/tools/go/bin.
set -euo pipefail

REPO="${REPO:-/home/<user>/work/phase4a}"
BASELINE="${BASELINE:-/home/<user>/work/phase3a-baseline}"   # detached at 99c70908
MAC="${MAC:-192.0.2.10}"
OLLAMA_URL="http://${MAC}:11434/api/chat"
EMBED_URL="http://${MAC}:8077"
export PATH="/home/<user>/tools/go/bin:${PATH}"
export GOTOOLCHAIN=local

R="${REPO}/reports/phase4/part1/RAW_EVIDENCE"
BIN="${R}/bin"
mkdir -p "${BIN}"

echo "== 0. locked-fixture integrity =="
python3 "${REPO}/reports/phase3/heldout/validate_lock.py"

echo "== 1. build arm binaries =="
echo PLACEHOLDER > "${REPO}/cli/beacon/internal/embedded/hooks.bin"
( cd "${REPO}/cli/beacon" && go build -o "${BIN}/beacon-v6" . )
( cd "${BASELINE}/cli/beacon" && go build -o "${BIN}/beacon-original" . )
( cd "${REPO}/cli/beacon" && go build -o "${BIN}/phase4aseed" ./internal/learning/testdata/phase3dseed )

echo "== 2. Benchmark A: harness validation (scripted) =="
rm -rf /tmp/p4-ref /tmp/p4-noop /tmp/harness-val
python3 "${REPO}/reports/phase3/part4/harness/agent_eval.py" --run-root /tmp/p4-ref \
  --fixture "${REPO}/reports/phase3/heldout/agent_tasks_v1.json" \
  --original "${BIN}/beacon-original" --v6 "${BIN}/beacon-v6" --seed "${BIN}/phase4aseed" \
  --backend scripted --script reference --output "${R}/validation_reference.json"
python3 "${REPO}/reports/phase3/part4/harness/agent_eval.py" --run-root /tmp/p4-noop \
  --fixture "${REPO}/reports/phase3/heldout/agent_tasks_v1.json" \
  --original "${BIN}/beacon-original" --v6 "${BIN}/beacon-v6" --seed "${BIN}/phase4aseed" \
  --backend scripted --script noop --output "${R}/validation_noop.json"
python3 "${REPO}/reports/phase4/part1/harness/validate_harness.py" \
  --workdir /tmp/harness-val --output "${R}/harness_validation.json" | tee "${R}/harness_validation.txt"

echo "== 3. Benchmark B: retrieval (Beacon lexical/hashed/hybrid + neural) =="
( cd "${REPO}/cli/beacon" && BEACON_PHASE3_RETRIEVAL=1 BEACON_EMBED_URL="${EMBED_URL}" \
    go test ./internal/learning/ -run TestPhase3ARetrieval -v -count=1 ) \
  | tee "${R}/retrieval_neural_mac.txt"
python3 "${REPO}/reports/phase4/part1/harness/retrieval_baselines.py" \
  --fixture "${REPO}/reports/phase3/heldout/retrieval_context_v1.json" \
  --embed-url "${EMBED_URL}" --scales 1000,10000,100000 \
  --output "${R}/retrieval_baselines.json" | tee "${R}/retrieval_baselines.txt"

echo "== 4. Benchmark C: local model capability =="
python3 "${REPO}/reports/phase4/part1/harness/model_capability_check.py" \
  --ollama-url "${OLLAMA_URL}" --model qwen2.5-coder:7b \
  --workdir /tmp/capcheck-7b --output "${R}/model_capability_qwen2.5-coder-7b.json"

echo "== 5. Benchmark D: six-task paired pilot =="
rm -rf /tmp/p4-ollama
python3 "${REPO}/reports/phase3/part4/harness/agent_eval.py" --run-root /tmp/p4-ollama \
  --fixture "${REPO}/reports/phase3/heldout/agent_tasks_v1.json" \
  --original "${BIN}/beacon-original" --v6 "${BIN}/beacon-v6" --seed "${BIN}/phase4aseed" \
  --backend ollama --ollama-url "${OLLAMA_URL}" --model qwen2.5-coder:7b \
  --output "${R}/six_task_pilot.json" | tee "${R}/six_task_pilot.txt"

echo "== 6. Benchmark F: reliability =="
( cd "${REPO}/cli/beacon" && go test ./internal/learning/ -count=1 ) | tee "${R}/reliability_full_learning.txt"
( cd "${REPO}/cli/beacon" && go test ./internal/hnsw/ -count=1 ) | tee "${R}/reliability_hnsw.txt"

echo "== 7. re-verify locked fixtures =="
python3 "${REPO}/reports/phase3/heldout/validate_lock.py"
echo "DONE"
