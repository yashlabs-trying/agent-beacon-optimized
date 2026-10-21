# Benchmark reproduction

## Hardware / software

- OS: WSL2 Ubuntu on Windows; 8 vCPU; 11th Gen Intel(R) Core(TM) i5-1155G7 @
  2.50GHz; 7.8 GB RAM.
- Go 1.27.0 linux/amd64; SQLite driver modernc.org/sqlite v1.59.0 (pure Go).
- Base commit `99c70908`; v2 `ee1cc516`; v3 `3065c210`; v4 branch from v3.
- Note: on Windows, Smart App Control blocks locally compiled binaries, so all
  execution was done in WSL. `go build`/`go vet` run anywhere.

## Datasets and seeds

- Lexical harness: `search_verify_test.go` `verifyDataset(n)` — deterministic,
  no RNG seed needed.
- Semantic labeled set: `semantic_test.go` `semanticCases()` (17 cases, four
  categories) — hand-authored; no training.
- ANN synthetic vectors: `ann_test.go` `syntheticVectors(n, dim, clusters, seed)`
  with seed 42 (recall) and 7 (benchmark); noise scaled as 1/sqrt(dim).

## Commands

```bash
cd cli/beacon

# correctness + race
go test ./internal/learning/...
go test -race ./internal/learning/...

# lexical scaling (v2 harness), 1K..1M
BEACON_VERIFY=1 BEACON_VERIFY_TREE=feature \
  BEACON_VERIFY_SIZES=1000,10000,100000,1000000 \
  go test -run TestVerifyHarness -v ./internal/learning/

# semantic quality
go test -run 'TestSemantic|TestHashed|TestHybrid' -v ./internal/learning/

# exact semantic latency
go test -run '^$' -bench BenchmarkSemanticExact -benchmem ./internal/learning/

# ANN vs exact (latency) and recall
go test -run TestANNRecall -v ./internal/learning/
BEACON_ANN_SIZES=1000,10000,100000 \
  go test -run '^$' -bench BenchmarkANNvsExact -benchmem ./internal/learning/

# trust
go test -run TestTrust -v ./internal/learning/

# context budgeting
go test -run TestContextBudgeting -v ./internal/learning/
```

Baseline comparison (original Beacon) is measured with the same lexical harness
copied into the untouched `99c70908` worktree with a no-op hooks file, run with
`BEACON_VERIFY_TREE=baseline`.

## Benchmark isolation

- Each benchmark builds its own temp database; cold vs warm are separated
  (`cold` resets the result cache; `warm` repeats a query).
- Baseline and optimized use identical datasets and query distributions.

## Reproduction requirements

- Go 1.27 toolchain; network access for module download on first build.
- coder/hnsw v0.6.1 in `go.mod` (v4 branch only).
- No model files, no hosted service.
