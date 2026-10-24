#!/usr/bin/env bash
# Phase 2 reproducibility script.
#
# Runs the Phase 2 HNSW, durability, trust, token/ranking and concurrency
# experiments and writes raw output under reports/phase2/raw/. It is portable to
# macOS and Linux, uses only the local Go toolchain, and never touches any
# Codex/Phase 1 worktree.
#
# Usage:
#   ./reports/phase2/run_phase2_benchmarks.sh            # short, ~5 min
#   SOAK_SECONDS=900 ./reports/phase2/run_phase2_benchmarks.sh   # + 15 min soak
#
# Environment:
#   GO_BIN        path to the go binary (default: go on PATH, else tools/go/bin/go)
#   RAW_DIR       output directory (default: reports/phase2/raw)
#   SOAK_SECONDS  if set, also run a sustained soak for this many seconds

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BEACON_DIR="$REPO_ROOT/cli/beacon"
RAW_DIR="${RAW_DIR:-$REPO_ROOT/reports/phase2/raw}"
SOAK_SECONDS="${SOAK_SECONDS:-0}"

if [[ -n "${GO_BIN:-}" ]]; then
  GO="$GO_BIN"
elif command -v go >/dev/null 2>&1; then
  GO="go"
elif [[ -x "$HOME/tools/go/bin/go" ]]; then
  GO="$HOME/tools/go/bin/go"
else
  echo "go toolchain not found; set GO_BIN" >&2
  exit 1
fi

mkdir -p "$RAW_DIR"
echo "go: $("$GO" version)"
echo "raw dir: $RAW_DIR"
echo "git sha: $(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
echo "uname: $(uname -a)"

cd "$BEACON_DIR"
# A placeholder embedded hooks binary is enough for the learning package tests.
if [[ ! -f internal/embedded/hooks.bin ]]; then
  echo "PLACEHOLDER" > internal/embedded/hooks.bin
fi

run() {
  local name="$1"; shift
  echo "=== $name ==="
  "$@" 2>&1 | tee "$RAW_DIR/$name.txt"
}

# HNSW construction throughput at the required sizes.
run hnsw_build_timing env BEACON_PHASE2_TIMING=1 BEACON_PHASE2_ANN_SIZES=1000,10000,25000,50000 \
  "$GO" test ./internal/learning/ -run TestHNSWBuildTiming -v -count=1 -timeout 20m

# HNSW recall vs exact nearest neighbours.
run hnsw_recall env BEACON_ANN_SIZES=10000,25000 \
  "$GO" test ./internal/learning/ -run '^$' -bench 'BenchmarkANNvsExact/n=(10000|25000)/ann' -benchtime=1x -count=1 -timeout 20m

# Durability / crash injection.
run durability "$GO" test ./internal/learning/ -run 'TestPhase2Crash|TestPhase2RepeatedRecovery|TestPhase2FaultHook' -v -count=1 -timeout 5m

# Adversarial trust and security.
run trust_adversarial "$GO" test ./internal/learning/ -run 'TestPhase2Adversarial|TestPhase2CrossScope|TestPhase2Unapproved|TestPhase2Hostile|TestPhase2TrustAnnotationLatency' -v -count=1 -timeout 5m

# Token and ranking evaluation.
run token_ranking "$GO" test ./internal/learning/ -run 'TestPhase2RankingAlternatives|TestPhase2TokenSelectionStrategies' -v -count=1 -timeout 5m

# Production integration.
run integration "$GO" test ./internal/learning/ -run 'TestPhase2IdempotentInit|TestPhase2Upgrade|TestPhase2IndexRebuild|TestPhase2Reopen|TestPhase2ANNLifecycle|TestPhase2EmbeddingModelVersion|TestPhase2LargeMemory|TestPhase2CorruptRow' -v -count=1 -timeout 5m

# Concurrency matrix (short).
for c in 1 4 8 16 32; do
  run "concurrency_$c" env BEACON_PHASE2_STRESS=1 BEACON_PHASE2_CONCURRENCY="$c" BEACON_PHASE2_SECONDS=10 \
    "$GO" test ./internal/learning/ -run TestPhase2ConcurrencyStress -v -count=1 -timeout 5m
done

if [[ "$SOAK_SECONDS" -gt 0 ]]; then
  run soak env BEACON_PHASE2_STRESS=1 BEACON_PHASE2_CONCURRENCY=8 BEACON_PHASE2_SECONDS="$SOAK_SECONDS" \
    "$GO" test ./internal/learning/ -run TestPhase2ConcurrencyStress -v -count=1 -timeout "$((SOAK_SECONDS + 300))s"
fi

echo "done. raw output in $RAW_DIR"
