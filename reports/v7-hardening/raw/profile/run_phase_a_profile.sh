#!/usr/bin/env bash
set -euo pipefail

source_root=/home/<user>/work/beacon-v7-hardening-profile
module_root="$source_root/cli/beacon"
evidence_root=/mnt/c/Users/yashs/work/beacon-v7-hardening/reports/v7-hardening/raw/profile
go_bin=/home/<user>/sdk/go/bin/go

cd "$module_root"
test "$(git rev-parse HEAD)" = 7b605aa84f0f570ba265fbda17fe6d99e927b10e
cp "$evidence_root/phase_a_profile_test.go" internal/learning/phase_a_profile_test.go
for output in environment.txt benchmark.log semantic_first.log direct_cpu.pprof direct_alloc.pprof direct_block.pprof direct_mutex.pprof direct.trace direct_profile.log semantic_cpu.pprof semantic_alloc.pprof semantic_block.pprof semantic_mutex.pprof semantic.trace semantic_profile.log direct_cpu_top.txt direct_alloc_top.txt semantic_cpu_top.txt semantic_alloc_top.txt; do
  test ! -e "$evidence_root/$output"
done

{
  date -u --iso-8601=seconds
  git rev-parse HEAD
  "$go_bin" version
  uname -a
  lscpu | sed -n '1,20p'
  free -h
  sha256sum "$evidence_root/phase_a_profile_test.go"
} > "$evidence_root/environment.txt"

"$go_bin" test ./internal/learning -run '^$' -bench '^BenchmarkProfileV7(Direct|List|SemanticWarm)$' -benchtime=1s -benchmem -count=3 | tee "$evidence_root/benchmark.log"
"$go_bin" test ./internal/learning -run '^TestProfileV7SemanticFirst$' -v -count=3 | tee "$evidence_root/semantic_first.log"

"$go_bin" test ./internal/learning -run '^$' -bench '^BenchmarkProfileV7Direct$' -benchtime=5s -cpuprofile "$evidence_root/direct_cpu.pprof" -memprofile "$evidence_root/direct_alloc.pprof" -blockprofile "$evidence_root/direct_block.pprof" -mutexprofile "$evidence_root/direct_mutex.pprof" -trace "$evidence_root/direct.trace" | tee "$evidence_root/direct_profile.log"
"$go_bin" test ./internal/learning -run '^$' -bench '^BenchmarkProfileV7SemanticWarm$' -benchtime=5s -cpuprofile "$evidence_root/semantic_cpu.pprof" -memprofile "$evidence_root/semantic_alloc.pprof" -blockprofile "$evidence_root/semantic_block.pprof" -mutexprofile "$evidence_root/semantic_mutex.pprof" -trace "$evidence_root/semantic.trace" | tee "$evidence_root/semantic_profile.log"

"$go_bin" tool pprof -top -nodecount=30 "$evidence_root/direct_cpu.pprof" > "$evidence_root/direct_cpu_top.txt"
"$go_bin" tool pprof -top -alloc_space -nodecount=30 "$evidence_root/direct_alloc.pprof" > "$evidence_root/direct_alloc_top.txt"
"$go_bin" tool pprof -top -nodecount=30 "$evidence_root/semantic_cpu.pprof" > "$evidence_root/semantic_cpu_top.txt"
"$go_bin" tool pprof -top -alloc_space -nodecount=30 "$evidence_root/semantic_alloc.pprof" > "$evidence_root/semantic_alloc_top.txt"
