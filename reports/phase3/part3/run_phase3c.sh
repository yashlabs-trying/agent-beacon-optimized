#!/usr/bin/env bash
# Phase 3C reproducibility script. Raw output under reports/phase3/part3/RAW_EVIDENCE/.
# Native Linux (WSL2 ok), Go 1.27.x. Neural is BLOCKED unless BEACON_EMBED_URL is live.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
BEACON_DIR="$REPO_ROOT/cli/beacon"
RAW_DIR="$REPO_ROOT/reports/phase3/part3/RAW_EVIDENCE"
mkdir -p "$RAW_DIR"

if command -v go >/dev/null 2>&1; then GO=go
elif [ -x "$HOME/tools/go/bin/go" ]; then GO="$HOME/tools/go/bin/go"
else echo "go not found" >&2; exit 1; fi

cd "$BEACON_DIR"
make build-hooks-current >/dev/null 2>&1 || echo PLACEHOLDER > internal/embedded/hooks.bin
run() { local n="$1"; shift; echo "=== $n ==="; "$@" 2>&1 | tee "$RAW_DIR/$n.txt"; }

{ echo "### gofmt -l"; gofmt -l .; echo "### build"; "$GO" build ./...; echo "build=$?";
  echo "### vet"; "$GO" vet ./...; echo "vet=$?"; } | tee "$RAW_DIR/build_vet_fmt.txt"

# Lifecycle / pooling tests
run lifecycle_tests "$GO" test -race ./internal/learning/ \
  -run 'TestStoreReusesOnePool|TestStoreCloseIsIdempotentAndBlocksUse|TestMultipleStoresIndependent|TestConcurrentStoreInitRaceFree|TestConcurrentStoresSamePathInitRaceFree|TestFailedInitializationDoesNotLeakPool|TestConcurrentReadsWritesThenClose' \
  -v -count=1 -timeout 5m

# Writer latency (after)
run after_writer env BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=10 \
  BEACON_PHASE3_W_REPS=3 BEACON_PHASE3_W_CHURN=0 BEACON_PHASE3_W_MODE=mixed \
  "$GO" test ./internal/learning/ -run TestPhase3BWriterLatency -v -count=1 -timeout 8m
run writer_conn_cost env BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=5 \
  "$GO" test ./internal/learning/ -run 'TestPhase3BConnectionMaterialised|TestPhase3BConnectionCost' -v -count=1

# Diagnostics
run checkpoint_probe env BEACON_PHASE3C_PROBE=1 "$GO" test ./internal/learning/ \
  -run 'TestPhase3CCheckpointProbe|TestPhase3CWritePhaseProfile' -v -count=1 -timeout 5m

# Validation
run learning_full "$GO" test ./internal/learning/ -count=1 -timeout 20m
run race_learning_hnsw "$GO" test -race -p 2 -count=1 -timeout 40m ./internal/learning/ ./internal/hnsw/
run full_cli_suite "$GO" test -p 2 -count=1 -timeout 30m ./...

echo "done. raw output in $RAW_DIR"
