#!/usr/bin/env bash
# Phase 4C reproduction script. Builds the layer binaries, dumps selection, runs the
# dev pilot with ablations and the held-out comparison, and the memory-history test.
set -euo pipefail

REPO="${REPO:-/home/<user>/work/phase4c}"
BASELINE="${BASELINE:-/home/<user>/work/phase3a-baseline}"   # detached at 99c70908
MAC="${MAC:-192.0.2.10}"
OLLAMA_URL="http://${MAC}:11434/api/chat"
EMBED_URL="http://${MAC}:8077"
export PATH="/home/<user>/tools/go/bin:${PATH}"
export GOTOOLCHAIN=local

R="${REPO}/reports/phase4/part3/RAW_EVIDENCE"
BIN="${R}/bin"
FIX="${REPO}/reports/phase4/part3/fixtures/phase4c_dev_v1.json"
HELD="${REPO}/reports/phase4/part3/fixtures/phase4c_heldout_v1.json"
mkdir -p "${BIN}"

echo "== 0. verify locked Phase 3 fixtures =="
python3 "${REPO}/reports/phase3/heldout/validate_lock.py"

echo "== 1. build + validate corpora =="
python3 "${REPO}/reports/phase4/part3/harness/build_dev_corpus.py" --out "${FIX}" --manifest "${R}/dev_manifest.json"
python3 "${REPO}/reports/phase4/part3/harness/build_heldout.py" --out "${HELD}" --manifest "${R}/heldout_manifest.json"

echo "== 2. build binaries =="
echo PLACEHOLDER > "${REPO}/cli/beacon/internal/embedded/hooks.bin"
( cd "${REPO}/cli/beacon" && go build -o "${BIN}/beacon-v6" . )
( cd "${BASELINE}/cli/beacon" && go build -o "${BIN}/beacon-original" . )
( cd "${REPO}/cli/beacon" && go build -o "${BIN}/phase4cseed" ./internal/learning/testdata/phase4cseed )

echo "== 3. unit + history tests =="
( cd "${REPO}/cli/beacon" && go test ./internal/learning/ -run 'TestSelectContext_|TestPhase4CMemoryHistory' -v -count=1 ) | tee "${R}/unit_tests.txt"

echo "== 4. selection dumps =="
( cd "${REPO}/cli/beacon" && BEACON_PHASE4C_SELECT=1 BEACON_PHASE4C_FIXTURE="${FIX}" \
    BEACON_PHASE4C_OUT="${R}/selection_dump.json" BEACON_PHASE4C_K=8 BEACON_EMBED_URL="${EMBED_URL}" \
    go test ./internal/learning/ -run TestPhase4CSelect -count=1 ) | tee "${R}/selection_dump.txt"
( cd "${REPO}/cli/beacon" && BEACON_PHASE4C_SELECT=1 BEACON_PHASE4C_FIXTURE="${HELD}" \
    BEACON_PHASE4C_OUT="${R}/selection_dump_heldout.json" BEACON_PHASE4C_K=8 BEACON_EMBED_URL="${EMBED_URL}" \
    go test ./internal/learning/ -run TestPhase4CSelect -count=1 ) | tee "${R}/selection_dump_heldout.txt"

echo "== 5. dev pilot (arms + ablations) =="
rm -rf /tmp/p4c-runs
python3 "${REPO}/reports/phase4/part3/harness/agent_pilot.py" --fixture "${FIX}" \
  --selection "${R}/selection_dump.json" --run-root /tmp/p4c-runs --bin-dir "${BIN}" \
  --seed "${BIN}/phase4cseed" --ollama-url "${OLLAMA_URL}" --model qwen2.5-coder:7b \
  --output "${R}/agent_pilot.json" | tee "${R}/agent_pilot.txt"
python3 "${REPO}/reports/phase4/part3/harness/analyze_pilot.py" --runs "${R}/agent_pilot.json" \
  --output "${R}/analysis.json" --text "${R}/analysis.txt"

echo "== 6. held-out pilot =="
rm -rf /tmp/p4c-held
python3 "${REPO}/reports/phase4/part3/harness/agent_pilot.py" --fixture "${HELD}" \
  --selection "${R}/selection_dump_heldout.json" --run-root /tmp/p4c-held --bin-dir "${BIN}" \
  --seed "${BIN}/phase4cseed" --ollama-url "${OLLAMA_URL}" --model qwen2.5-coder:7b \
  --arms none,original,v6_semantic,v6_intelligent,oracle \
  --output "${R}/agent_pilot_heldout.json" | tee "${R}/agent_pilot_heldout.txt"
python3 "${REPO}/reports/phase4/part3/harness/analyze_pilot.py" --runs "${R}/agent_pilot_heldout.json" \
  --output "${R}/analysis_heldout.json" --text "${R}/analysis_heldout.txt"

echo "== 7. reliability regression =="
( cd "${REPO}/cli/beacon" && go test ./internal/learning/ -count=1 ) | tee "${R}/reliability_full.txt"
( cd "${REPO}/cli/beacon" && go test -race ./internal/learning/ -run 'Concurrency|Concurrent|PutMemoryWaits' -count=1 ) | tee "${R}/reliability_race.txt"

echo "== 8. re-verify locked fixtures =="
python3 "${REPO}/reports/phase3/heldout/validate_lock.py"
echo DONE
