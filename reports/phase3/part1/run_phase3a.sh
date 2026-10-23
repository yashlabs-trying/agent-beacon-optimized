#!/usr/bin/env bash
# Phase 3A reproducibility script.
#
# Reproduces every Phase 3A result on the isolated branch
# phase3a/beacon-v6-verification (base c9275d84). Raw output is written under
# reports/phase3/part1/RAW_EVIDENCE/.
#
# Host expectations: native Linux (WSL2 ok), Go 1.27.x on PATH, the locked
# fixture at reports/phase3/heldout/. The real-neural arm additionally needs a
# reachable BGE-small sidecar via BEACON_EMBED_URL (otherwise it is BLOCKED).
#
# Usage:  ./reports/phase3/part1/run_phase3a.sh
#         SOAK_SECONDS=900 ./reports/phase3/part1/run_phase3a.sh   # optional soak

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
BEACON_DIR="$REPO_ROOT/cli/beacon"
RAW_DIR="$REPO_ROOT/reports/phase3/part1/RAW_EVIDENCE"
mkdir -p "$RAW_DIR"

if command -v go >/dev/null 2>&1; then GO=go
elif [ -x "$HOME/tools/go/bin/go" ]; then GO="$HOME/tools/go/bin/go"
else echo "go not found; set PATH" >&2; exit 1; fi

echo "go: $("$GO" version)"; echo "uname: $(uname -a)"; echo "raw: $RAW_DIR"
cd "$BEACON_DIR"
[ -f internal/embedded/hooks.bin ] || { echo PLACEHOLDER > internal/embedded/hooks.bin; }

run() { local n="$1"; shift; echo "=== $n ==="; "$@" 2>&1 | tee "$RAW_DIR/$n.txt"; }

# --- static gates ---------------------------------------------------------
{ echo "### gofmt -l"; gofmt -l .; echo "### build"; "$GO" build ./...; echo "build=$?";
  echo "### vet"; "$GO" vet ./...; echo "vet=$?"; } | tee "$RAW_DIR/build_vet_fmt.txt"

# --- Workstream A: retrieval (neural BLOCKED unless BEACON_EMBED_URL set) --
run retrieval_v6 env BEACON_PHASE3_RETRIEVAL=1 "$GO" test ./internal/learning/ \
  -run 'TestPhase3ARetrieval' -v -count=1 -timeout 10m

# --- Workstream B: bounded ANN -------------------------------------------
run ann_vs_exact_m64 env BEACON_PHASE3_ANN=1 BEACON_PHASE3_ANN_SIZES=1000,10000,25000 BEACON_ANN_M=64 \
  "$GO" test ./internal/learning/ -run '^TestPhase3AANNvsExact$' -v -count=1 -timeout 25m
run ann_vs_exact_100k env BEACON_PHASE3_ANN=1 BEACON_PHASE3_ANN_SIZES=100000 BEACON_ANN_M=64 \
  "$GO" test ./internal/learning/ -run '^TestPhase3AANNvsExact$' -v -count=1 -timeout 30m
run ann_correctness "$GO" test ./internal/learning/ \
  -run 'TestANNRejectsInvalidVectors|TestHNSWDeterminism|TestANNInsertionOrderAndDuplicates' -v -count=1 -timeout 5m
run ann_lifecycle "$GO" test ./internal/learning/ \
  -run 'TestPhase2ANNLifecycleAcrossGenerations|TestPhase2EmbeddingModelVersionIsolation' -v -count=1 -timeout 5m

# --- Workstream C: contradiction + trust ---------------------------------
run contradiction_locked env BEACON_PHASE3_CONTRADICTION=1 "$GO" test ./internal/learning/ \
  -run 'TestPhase3AContradictionLocked' -v -count=1 -timeout 5m
run security_trust "$GO" test ./internal/learning/ \
  -run 'TestAdversarialMemoryRetrieval|TestPhase2CrossScopeLeakageAllPaths|TestPhase2UnapprovedEvidenceNotVisible|TestPhase2HostileMemoryContent|TestPhase2TrustAnnotationLatency|TestTrustContradictionPrecisionRecall|TestTrustPhase1LabeledScenarios|TestPhase2AdversarialContradictionMatrix|TestTrustNoCrossScopeLeakage' \
  -v -count=1 -timeout 10m

# --- Workstream D: token budget ------------------------------------------
run token_budget "$GO" test ./internal/learning/ \
  -run 'TestPhase2RankingAlternatives|TestPhase2TokenSelectionStrategies|TestContextBudgeting|TestSelectContextActuallyTruncatesBody' \
  -v -count=1 -timeout 10m

# --- Workstream E: durability + concurrency ------------------------------
run durability_crash "$GO" test ./internal/learning/ \
  -run 'TestPhase2Crash|TestPhase2RepeatedRecovery|TestPhase2FaultHook|TestPutMemoryProcessCrashRecovery|TestPutMemoryWaitsForBoundedWriteContention' \
  -v -count=1 -timeout 10m
for c in 1 4 8 16; do
  run "concurrency_$c" env BEACON_PHASE2_STRESS=1 BEACON_PHASE2_CONCURRENCY="$c" BEACON_PHASE2_SECONDS=10 \
    "$GO" test ./internal/learning/ -run 'TestPhase2ConcurrencyStress|TestPhase2CacheInvalidationUnderWrites' -v -count=1 -timeout 5m
done

# --- Race + full suite ----------------------------------------------------
run race_learning_hnsw "$GO" test -race -p 2 -count=1 -timeout 40m ./internal/learning/ ./internal/hnsw/
make build-hooks-current
run full_cli_suite "$GO" test -p 2 -count=1 -timeout 30m ./...

if [ "${SOAK_SECONDS:-0}" -gt 0 ]; then
  run soak env BEACON_PHASE2_STRESS=1 BEACON_PHASE2_CONCURRENCY=8 BEACON_PHASE2_SECONDS="$SOAK_SECONDS" \
    "$GO" test ./internal/learning/ -run TestPhase2ConcurrencyStress -v -count=1 -timeout "$((SOAK_SECONDS + 300))s"
fi

echo "done. raw output in $RAW_DIR"
