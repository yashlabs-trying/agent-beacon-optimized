#!/usr/bin/env bash
# Phase 3D reproducibility script.
#
# Evaluation A (neural) is BLOCKED unless a BGE-small sidecar is reachable at
# BEACON_EMBED_URL. Evaluation B (six-task agent) is BLOCKED unless a local
# Ollama model is running; the harness is validated with --backend scripted.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
BEACON_DIR="$REPO_ROOT/cli/beacon"
RAW_DIR="$REPO_ROOT/reports/phase3/part4/RAW_EVIDENCE"
FIX="$REPO_ROOT/reports/phase3/heldout/agent_tasks_v1.json"
HAR="$REPO_ROOT/reports/phase3/part4/harness/agent_eval.py"
BIN="$RAW_DIR/bin"
mkdir -p "$BIN" "$RAW_DIR"

if command -v go >/dev/null 2>&1; then GO=go
elif [ -x "$HOME/tools/go/bin/go" ]; then GO="$HOME/tools/go/bin/go"
else echo "go not found" >&2; exit 1; fi

cd "$BEACON_DIR"
[ -f internal/embedded/hooks.bin ] || echo PLACEHOLDER > internal/embedded/hooks.bin
"$GO" build -o "$BIN/beacon-v6" .
"$GO" build -o "$BIN/phase3dseed" ./internal/learning/testdata/phase3dseed

# Evaluation A: neural (BLOCKED unless a sidecar is reachable)
if [ -n "${BEACON_EMBED_URL:-}" ]; then
  BEACON_PHASE3_RETRIEVAL=1 "$GO" test ./internal/learning/ -run TestPhase3ARetrieval -v -count=1 \
    2>&1 | tee "$RAW_DIR/neural_retrieval.txt"
else
  echo "NEURAL BLOCKED: set BEACON_EMBED_URL to a running BGE-small sidecar" | tee "$RAW_DIR/neural_retrieval.txt"
fi

# Evaluation B: harness validation (deterministic; not a model outcome)
rm -rf "$RAW_DIR/runs"
python3 "$HAR" --run-root "$RAW_DIR/runs/reference" --fixture "$FIX" \
  --original "$BIN/beacon-original" --v6 "$BIN/beacon-v6" --seed "$BIN/phase3dseed" \
  --backend scripted --script reference --output "$RAW_DIR/validation_reference.json" \
  2>&1 | tee "$RAW_DIR/validation_reference.txt"
python3 "$HAR" --run-root "$RAW_DIR/runs/noop" --fixture "$FIX" \
  --original "$BIN/beacon-original" --v6 "$BIN/beacon-v6" --seed "$BIN/phase3dseed" \
  --backend scripted --script noop --output "$RAW_DIR/validation_noop.json" \
  2>&1 | tee "$RAW_DIR/validation_noop.txt"

# Real model run (only if Ollama is up)
if curl -sS -o /dev/null http://127.0.0.1:11434/api/tags 2>/dev/null; then
  python3 "$HAR" --run-root "$RAW_DIR/runs/ollama" --fixture "$FIX" \
    --original "$BIN/beacon-original" --v6 "$BIN/beacon-v6" --seed "$BIN/phase3dseed" \
    --backend ollama --model "${BEACON_AGENT_MODEL:-qwen2.5-coder:7b}" \
    --output "$RAW_DIR/agent_eval_ollama.json" 2>&1 | tee "$RAW_DIR/agent_eval_ollama.txt"
else
  echo "AGENT MODEL BLOCKED: no Ollama at 127.0.0.1:11434" | tee "$RAW_DIR/agent_eval_blocked.txt"
fi

echo "done. raw output in $RAW_DIR"
