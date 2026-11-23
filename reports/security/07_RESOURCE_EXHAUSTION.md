# 07 — Resource Exhaustion and Overload Resistance (Security Layer 5)

Status: **bounded-work controls implemented and tested**; large-scale benchmarks
(100K/1M memories, 64 clients) **NOT TESTED** in this pass.

## Threat surface

Broad/repeated no-match searches, very large inputs, high-cardinality requests,
concurrent DB contention, cache stampedes, excessive JSON decoding, unbounded
memory, resource-heavy MCP calls.

## What changed

| Control | Where | Behavior |
|---|---|---|
| Bounded text scan | `learning/store.go` `listMemoriesScan` | `searchScanRowCap` (200,000 rows) caps the non-selective scan; exceeding it returns `ErrSearchScanExceeded` — an explicit overload error, never a silent truncation. |
| Bounded indexed fetch | `learning/search.go` (pre-existing) | `searchCandidateCap = 256` limits indexed candidates; the planner falls back to the bounded scan when a posting list overflows. |
| Query text limit | `mcpserver/server.go` | `q`/`task` > 4096 bytes rejected with a clear error. |
| Query term limit | `mcpserver/server.go` | > 32 terms rejected. |
| ID / project limits | `mcpserver/server.go` | memory id > 512 bytes, project identifier > 4096 bytes rejected. |
| One shared store | `mcpserver/server.go` | The server reuses a single `*sql.DB` pool (was: a fresh, never-closed pool per call). `Server.Close()` releases it. |
| HTTP body limit | `mcpserver/server.go` (pre-existing) | `http.MaxBytesReader` 4 MB on `/mcp`. |
| HTTP header timeout | `cmd/mcp.go` (pre-existing) | `ReadHeaderTimeout: 5s`. |
| Loopback + DNS-rebind | `dashboard.ValidateLoopbackAddr`, `RequireLoopbackHost` (pre-existing) | HTTP binds loopback only; non-loopback `Host` rejected. |

## Tests (all PASS)

```
go test ./internal/learning/ -run 'TestSearchScanCapReturnsExplicitError|TestSelectiveQueryStaysUnderCap' -count=1
--- PASS: TestSearchScanCapReturnsExplicitError
--- PASS: TestSelectiveQueryStaysUnderCap

go test ./internal/mcpserver/ -run 'TestResource' -count=1
--- PASS: TestResourceOversizedQueryRejected
--- PASS: TestResourceTooManyTermsRejected
--- PASS: TestResourceOversizedIDRejected
```

`TestSelectiveQueryStaysUnderCap` confirms the cap does not affect a selective
query (which uses the bounded indexed path).

## Finding C benchmark (original scan vs V7 indexed)

From `security_finding_c_test.go` (no-match query, same store):

| N | original scan | V7 indexed |
|---|---|---|
| 1000 | 8.05 ms, 12,035 allocs | 81 µs, 82 allocs |
| 5000 | 46.0 ms, 60,910 allocs | 142 µs, 82 allocs |

The indexed path is roughly flat in N and constant in allocations; the original
path grows linearly.

## Not tested / remaining

- **Request deadlines and context cancellation** are **not wired** through the
  query path. The scan is row-bounded but a single very large selective query is
  still executed to completion. Reported as **NOT TESTED / NOT IMPLEMENTED**.
- **Cache stampede** under concurrent cold misses: the existing cache is
  per-process with a generation check; no stampede test was run.
- **100K / 1M memory benchmarks and 64-client runs**: NOT TESTED (time and
  Windows Application Control constraints).
- **Backpressure / admission control**: only input-shape rejection is
  implemented; no queue or load-shedding.

## Acceptance

| Requirement | Status |
|---|---|
| Bounded work on the declared adversarial workload | PARTIAL (row/term/byte caps; no deadline) |
| Clear overload responses | PASS (`ErrSearchScanExceeded`, input errors) |
| No silent truncation | PASS (explicit error) |
| No new datastore/queue added | PASS (reused SQLite/FTS5) |
| Large-scale scalability characterization | **NOT TESTED** |
