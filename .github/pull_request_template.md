## Summary

What does this change and why?

## Type

- [ ] Optimization (performance, allocations, latency)
- [ ] Security / hardening
- [ ] Correctness fix
- [ ] Documentation / examples
- [ ] Tests / tooling

## Evidence (required for performance or security claims)

- What did you measure, on what host, with which command?
- Link the report under `reports/` or paste raw numbers with units.
- [ ] I did **not** present historical/unreproducible numbers as current results.
- [ ] I did **not** claim an agent-quality improvement from a retrieval microbenchmark.

## Checklist

- [ ] `cd cli/beacon && go build ./...` passes
- [ ] `go test ./internal/learning/ -count=1` passes
- [ ] `go vet ./internal/learning/` is clean
- [ ] Race subset passes if I touched concurrency:
      `go test -race ./internal/learning/ -run 'Concurrency|Concurrent|Crash'`
- [ ] New/changed behavior is documented (README or `docs/optimization/`)
- [ ] Public APIs and established default behavior are unchanged (or the change is
      explained and justified)
- [ ] Upstream attribution, license notices, and benchmark evidence are preserved
- [ ] No secrets or credentials are included

## Related

Closes #<!-- issue -->
