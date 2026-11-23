# Example B — Large-scale retrieval

Seed a realistic multi-hundred-memory store and run the optimized question-aware
retrieval path against it, then run the allocation benchmark that is the headline
retrieval result. Demonstrates **indexed retrieval** rather than a naive scan.

## Prereqs

Same as [Example A](../memory-basic/README.md): Go 1.27+, Python 3.10+, and a built
`beacon` binary.

## Run

```sh
cd cli/beacon && CGO_ENABLED=0 go build -o /tmp/beacon .
cd ../../examples/memory-scale
BEACON_BIN=/tmp/beacon python3 run.py --count 1000
```

## What it does

1. Builds a seeder that writes `--count` distinct memories (default 1000) through the
   real `Store.PutMemory`, including one "needle" fact among distractors.
2. Starts `beacon mcp serve` and calls `search_memory` for the needle, asserting the
   indexed path returns it without scanning the whole corpus.
3. Prints wall-clock retrieval time for the query at the chosen scale.

## Expected output

```
workspace: /tmp/beacon-example-B-XXXX
seeded:    1000 memories
search_memory (needle) -> {"memories":[{"id":"needle",...}], ...}
elapsed:   ~N ms for a 1000-memory store
RESULT: PASS — indexed retrieval returned the needle.
```

## Reproduce the allocation benchmark (the headline number)

```sh
cd cli/beacon
go test ./internal/learning/ -run '^$' -bench 'BenchmarkPhaseCSearchQuestionWarm' -benchmem -count=5
```

Expected: **~875 allocs/op**, ~130 KB/op (warm, 1,000-memory store, CandidateK 50). The
`ListMemories` (Limit 8) control is ~58 µs / 64 allocs.

## Notes and limits

- Wall-clock varies >2× run-to-run on an I/O-bound host; **report allocations**, not a
  single latency. See [../../docs/optimization/benchmarks.md](../../docs/optimization/benchmarks.md).
- Put the database on a **local** filesystem; a synchronized mount makes every commit
  ~1 s. Set `BEACON_MEMORY_DIR` if needed.
- Scale above 1M memories and >32 concurrent writers is **not verified** — see
  [limitations](../../docs/optimization/limitations.md).
