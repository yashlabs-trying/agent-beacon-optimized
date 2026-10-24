#!/usr/bin/env bash
# Phase 3B reproducibility script. Writes raw output under
# reports/phase3/part2/RAW_EVIDENCE/. Native Linux (WSL2 ok), Go 1.27.x.
# Neural is BLOCKED unless a BGE-small sidecar is reachable at BEACON_EMBED_URL.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
BEACON_DIR="$REPO_ROOT/cli/beacon"
RAW_DIR="$REPO_ROOT/reports/phase3/part2/RAW_EVIDENCE"
mkdir -p "$RAW_DIR"

if command -v go >/dev/null 2>&1; then GO=go
elif [ -x "$HOME/tools/go/bin/go" ]; then GO="$HOME/tools/go/bin/go"
else echo "go not found" >&2; exit 1; fi

cd "$BEACON_DIR"
[ -f internal/embedded/hooks.bin ] || { make build-hooks-current >/dev/null 2>&1 || echo PLACEHOLDER > internal/embedded/hooks.bin; }
run() { local n="$1"; shift; echo "=== $n ==="; "$@" 2>&1 | tee "$RAW_DIR/$n.txt"; }

{ echo "### gofmt -l"; gofmt -l .; echo "### build"; "$GO" build ./...; echo "build=$?";
  echo "### vet"; "$GO" vet ./...; echo "vet=$?"; } | tee "$RAW_DIR/build_vet_fmt.txt"

# Contradiction
run contradiction_dev_and_locked env BEACON_PHASE3_CONTRADICTION=1 "$GO" test ./internal/learning/ \
  -run 'TestPhase3BContradictionDev|TestPhase3BLockedContradictionRegression' -v -count=1
run contradiction_locked_crosstalk env BEACON_PHASE3_CONTRADICTION=1 "$GO" test ./internal/learning/ \
  -run TestPhase3AContradictionLocked -v -count=1
run security_trust "$GO" test ./internal/learning/ \
  -run 'TestAdversarialMemoryRetrieval|TestPhase2CrossScopeLeakageAllPaths|TestPhase2UnapprovedEvidenceNotVisible|TestPhase2HostileMemoryContent|TestTrustContradictionPrecisionRecall|TestTrustPhase1LabeledScenarios|TestPhase2AdversarialContradictionMatrix|TestTrustNoCrossScopeLeakage' -v -count=1

# Writer latency
run writer_repro_existing env BEACON_PHASE2_STRESS=1 BEACON_PHASE2_CONCURRENCY=16 BEACON_PHASE2_SECONDS=10 \
  "$GO" test ./internal/learning/ -run TestPhase2ConcurrencyStress -v -count=1
run writer_connection_materialised env BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=5 \
  "$GO" test ./internal/learning/ -run TestPhase3BConnectionMaterialised -v -count=1
run writer_synchronous env BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=5 \
  "$GO" test ./internal/learning/ -run TestPhase3BSynchronousCost -v -count=1
run writer_matrix env BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=10 \
  BEACON_PHASE3_W_REPS=2 BEACON_PHASE3_W_CHURN=0 BEACON_PHASE3_W_MODE=mixed \
  "$GO" test ./internal/learning/ -run TestPhase3BWriterLatency -v -count=1

# Token budget
run token_analysis "$GO" test ./internal/learning/ \
  -run 'TestPhase2TokenSelectionStrategies|TestPhase3BTokenLostFindings|TestPhase3BEvidenceDedupPreservesFindings' -v -count=1

# Validation
run learning_full "$GO" test ./internal/learning/ -count=1 -timeout 20m
run race_learning_hnsw "$GO" test -race -p 2 -count=1 -timeout 40m ./internal/learning/ ./internal/hnsw/
run full_cli_suite "$GO" test -p 2 -count=1 -timeout 30m ./...

echo "done. raw output in $RAW_DIR"
