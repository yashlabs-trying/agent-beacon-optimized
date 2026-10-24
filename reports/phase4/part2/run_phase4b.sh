#!/usr/bin/env bash
# Phase 4B reproduction script (development calibration pilot).
#
# Runs the adversarial dev corpus across all arms with a real local model on the
# Apple M5 Mac. Does not download models, use paid compute, or push.
set -euo pipefail

REPO="${REPO:-/home/<user>/work/phase4b}"
BASELINE="${BASELINE:-/home/<user>/work/phase3a-baseline}"   # detached at 99c70908
MAC="${MAC:-192.0.2.10}"
OLLAMA_URL="http://${MAC}:11434/api/chat"
EMBED_URL="http://${MAC}:8077"
export PATH="/home/<user>/tools/go/bin:${PATH}"
export GOTOOLCHAIN=local

R="${REPO}/reports/phase4/part2/RAW_EVIDENCE"
BIN="${R}/bin"
FIX="${REPO}/reports/phase4/part2/fixtures/phase4b_dev_v1.json"
mkdir -p "${BIN}"

echo "== 0. verify Phase 3 locked fixtures =="
python3 "${REPO}/reports/phase3/heldout/validate_lock.py"

echo "== 1. build + validate dev corpus =="
python3 "${REPO}/reports/phase4/part2/harness/build_dev_corpus.py" \
  --out "${FIX}" --manifest "${R}/dev_manifest.json"

echo "== 2. build arm binaries =="
echo PLACEHOLDER > "${REPO}/cli/beacon/internal/embedded/hooks.bin"
( cd "${REPO}/cli/beacon" && go build -o "${BIN}/beacon-v6" . )
( cd "${BASELINE}/cli/beacon" && go build -o "${BIN}/beacon-original" . )
( cd "${REPO}/cli/beacon" && go build -o "${BIN}/phase4bseed" ./internal/learning/testdata/phase4bseed )

echo "== 3. in-process retrieval dump (lexical/hashed/hybrid/neural) =="
( cd "${REPO}/cli/beacon" && \
  BEACON_PHASE4B_RETRIEVAL=1 BEACON_PHASE4B_FIXTURE="${FIX}" \
  BEACON_PHASE4B_OUT="${R}/retrieval_dump.json" BEACON_PHASE4B_K=5 \
  BEACON_EMBED_URL="${EMBED_URL}" \
  go test ./internal/learning/ -run TestPhase4BRetrieval -v -count=1 ) \
  | tee "${R}/retrieval_dump.txt"

echo "== 4. agent pilot (all arms) =="
rm -rf /tmp/p4b-runs
python3 "${REPO}/reports/phase4/part2/harness/agent_pilot.py" \
  --fixture "${FIX}" --retrieval "${R}/retrieval_dump.json" \
  --run-root /tmp/p4b-runs --bin-dir "${BIN}" --seed "${BIN}/phase4bseed" \
  --ollama-url "${OLLAMA_URL}" --model qwen2.5-coder:7b \
  --output "${R}/agent_pilot.json" | tee "${R}/agent_pilot.txt"

echo "== 5. paired analysis =="
python3 "${REPO}/reports/phase4/part2/harness/analyze_pilot.py" \
  --runs "${R}/agent_pilot.json" --output "${R}/analysis.json" --text "${R}/analysis.txt"

echo "== 6. re-verify locked fixtures =="
python3 "${REPO}/reports/phase3/heldout/validate_lock.py"
echo "DONE"
