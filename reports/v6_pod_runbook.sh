#!/usr/bin/env bash
# Beacon V6 GPU-server runbook. Run this ON the RunPod RTX 3090 (Linux).
# It installs Go 1.27 if needed, checks out the branch, runs the test suite,
# then the 1M lexical and 1M ANN benchmarks. Results land in $RES.
#
# Usage:  bash v6_pod_runbook.sh [branch]
set -euo pipefail

BRANCH="${1:-research/beacon-memory-engine-v5}"
WORK="${WORK:-$HOME/beacon-v6}"
RES="${RES:-$HOME/v6-results}"
mkdir -p "$RES"

echo "== environment ==" | tee "$RES/environment.txt"
{ uname -a; nvidia-smi -L; nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv,noheader;
  nproc; free -g; df -h /; } 2>&1 | tee -a "$RES/environment.txt"

if ! command -v go >/dev/null 2>&1 || ! go version | grep -q "go1.27"; then
  curl -fsSL -o /tmp/go.tgz https://go.dev/dl/go1.27.0.linux-amd64.tar.gz
  rm -rf "$HOME/sdk"; mkdir -p "$HOME/sdk"; tar -C "$HOME/sdk" -xzf /tmp/go.tgz
fi
export PATH="$HOME/sdk/go/bin:$PATH"; export GOTOOLCHAIN=local
go version 2>&1 | tee -a "$RES/environment.txt"

if [ ! -d "$WORK" ]; then
  git clone https://github.com/Asymptote-Labs/agent-beacon.git "$WORK"
fi
cd "$WORK"
git fetch origin "$BRANCH"
git checkout "$BRANCH"
git log -1 --format='%H %s' 2>&1 | tee -a "$RES/environment.txt"

cd cli/beacon
echo "== build/vet/test ==" | tee "$RES/tests.txt"
go build ./... 2>&1 | tee -a "$RES/tests.txt"
go vet ./internal/learning/... 2>&1 | tee -a "$RES/tests.txt"
go test -count=1 ./internal/learning/... 2>&1 | tee -a "$RES/tests.txt"

echo "== 1M lexical scaling ==" | tee "$RES/lexical_1m.txt"
BEACON_VERIFY=1 BEACON_VERIFY_TREE=pod BEACON_VERIFY_SIZES=1000000 \
  go test -run TestVerifyHarness -v -count=1 -timeout 90m ./internal/learning/ 2>&1 | tee -a "$RES/lexical_1m.txt"

echo "== 1M ANN (M=64) ==" | tee "$RES/ann_1m.txt"
BEACON_ANN_SIZES=1000000 go test -run '^$' -bench BenchmarkANNvsExact -benchmem -benchtime=1x -count=1 -timeout 180m ./internal/learning/ 2>&1 | tee -a "$RES/ann_1m.txt"

echo "DONE. Results in $RES"
