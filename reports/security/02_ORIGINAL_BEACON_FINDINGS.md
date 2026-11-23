# 02 — Original Beacon Findings (reproduced)

Frozen original: `99c709081ad49da1c5ade205929b61b19360ed9b`.
V7 baseline: `7c17c5fd1503769e0fc147c0f9d3c551ee89c539`.

Cross-check that the reproduction targets are unchanged between the two commits:

```
git diff --stat 99c70908 7c17c5fd -- cli/beacon/internal/mcpserver/server.go
git diff --stat 99c70908 7c17c5fd -- cli/beacon/internal/learning/candidate.go
git diff --stat 99c70908 7c17c5fd -- cli/beacon/internal/learning/store.go
```

Result: `server.go` and `candidate.go` are **byte-identical** between the frozen
original and V7 (empty diff). `store.go` differs (the V7 FTS5 search work).
Therefore Findings A and B reproduce identically on both commits, and the V7
baseline run is a faithful proxy for the original. Finding C is reproduced
against the original algorithm, which V7 retains verbatim as
`listMemoriesScan`.

---

## Finding A — Project access boundaries

**Classification: CONDITIONAL RISK → CONFIRMED AUTHORIZATION DEFECT in a
multi-scope deployment. Patch warranted: YES (implemented).**

| Field | Value |
|---|---|
| Source | `cli/beacon/internal/mcpserver/server.go` — `get_memory` handler (~line 337) and `parseMemoryQuery` (~line 386) |
| Also | `cli/beacon/internal/learning/store.go` — `GetMemory(id)` (~line 834), unscoped |
| Frozen original | `99c70908` |
| Trust boundary | The MCP tool-call boundary. The caller supplies `project_id` / `project`; `get_memory` supplies only an `id`. |
| Attacker prerequisites | Ability to call the memory tools. In stdio mode this is any local process under the store owner; in loopback HTTP mode any local process that can reach `127.0.0.1:8766`. |
| Minimal repro | `cli/beacon/internal/mcpserver/security_baseline_repro_test.go` (`TestBaselineGetMemoryLeaksAcrossProjects`, `TestBaselineSearchTrustsCallerProjectID`) |
| Observed | `get_memory` returned a memory whose project the caller never named; `search_memory` honored an arbitrary caller-supplied `project_id`. |
| Impact | Confidentiality: cross-project memory disclosure. |
| Original mitigation | None for authorization. `ProjectIDForPath` stops a request-supplied path from reaching a filesystem expression and maps an unknown project to an ID that matches no rows — a path-safety control, **not** an authorization control. |
| V7 status | Identical at baseline; V7 added no authorization. |
| Patch | Implemented in `codex/beacon-v7-security-hardening` (see report 03). |

Raw observed output (V7 baseline, default mode):

```
=== RUN   TestBaselineGetMemoryLeaksAcrossProjects
    BASELINE FINDING A CONFIRMED: get_memory returned another project's memory:
    {"schema_version":"beacon.learning.v1","id":"memory-secret",
     "candidate_id":"candidate-memory-secret","kind":"convention",
     "title":"Secret project convention","body":"Internal credential rotation procedure.",
     "project":{"id":"project-secret"}, ...}
--- PASS: TestBaselineGetMemoryLeaksAcrossProjects
=== RUN   TestBaselineSearchTrustsCallerProjectID
    BASELINE FINDING A CONFIRMED: search_memory honored an arbitrary caller project_id:
    {"memories":[{"id":"memory-secret",...}],"returned":1,"limit":5}
--- PASS: TestBaselineSearchTrustsCallerProjectID
```

Note the disclosure of the full body (`Internal credential rotation procedure.`).
This is a **CONDITIONAL RISK** in the documented single-user local deployment
(one OS user, one trust boundary), and a **CONFIRMED AUTHORIZATION DEFECT** as
soon as scopes are meant to be isolated (multiple agents, or a shared HTTP
endpoint). It is not claimed as a remote vulnerability.

---

## Finding B — Multi-record memory transitions

**Classification: CONFIRMED RELIABILITY DEFECT (atomicity gap). Patch
warranted: YES (implemented in Phase 4).**

| Field | Value |
|---|---|
| Source | `cli/beacon/internal/learning/candidate.go` — `ApproveCandidateWithEdits` (~line 131), `SupersedeCandidate` (~line 213), `RejectCandidate` (~line 193) |
| Frozen original | `99c70908` (byte-identical to V7 baseline) |
| Trust boundary | Reviewer transition boundary (approve/reject/supersede). |
| Attacker prerequisites | None required for the reliability defect; a crash or a concurrent reviewer suffices. |
| Minimal repro | Planned `security_lifecycle_test.go` (Phase 4). Baseline observation is by source inspection plus `phase2_crash_injection_test.go`. |
| Observed | Approval writes the authoritative memory via `PutMemory` (its own transaction) and then the candidate via `PutCandidate` (a separate statement, not covered by `writeMu`). A crash between the two leaves an approved memory whose candidate is still in `candidate` state; a retry re-runs approval. `SupersedeCandidate` writes the candidate first, then the superseded memory, in separate operations. The candidate-state check and the writes are not in one transaction (TOCTOU). |
| Impact | Integrity / availability: observable intermediate review state; inconsistent audit history after a crash; concurrent reviewers race the state check. |
| Original mitigation | Deterministic IDs make `PutMemory`/`PutCandidate` idempotent, which limits duplication but does not make the pair atomic. |
| V7 status | Same at baseline. |
| Patch | Implemented in Phase 4 (single transaction). |

---

## Finding C — Expensive text-search behavior

**Classification: PERFORMANCE RISK in the original / MITIGATED in V7 (with a
residual bound to characterize).**

| Field | Value |
|---|---|
| Source | `cli/beacon/internal/learning/store.go` — original `ListMemories` (text branch) |
| Frozen original | `99c70908` |
| Trust boundary | Query surface (`search_memory`, `get_memory_context`). |
| Attacker prerequisites | Ability to issue text queries (same as Finding A). |
| Minimal repro | `cli/beacon/internal/learning/security_finding_c_test.go` (`BenchmarkFindingCSearchNoMatch`) |
| Observed | Original text path issues `SELECT memory_json FROM memories WHERE <scope> ORDER BY updated_at DESC` with **no LIMIT**, JSON-decodes every scoped row, and filters in Go. Cost is linear in the project's memory count, including for no-match queries. |
| Impact | Availability / cost: a broad or repeated no-match query does O(N) decode work and allocations. |
| Original mitigation | None. |
| V7 status | **MITIGATED**: V7 keeps the scan as `listMemoriesScan` (correctness reference) and adds an FTS5-backed `searchMemories`. |
| Patch | Retain and characterize; add bounded-work controls in Phase 6. |

Measured (20 iterations, `-test.benchtime=20x`, same store, no-match query):

```
BenchmarkFindingCSearchNoMatch/original-scan/N=1000   20   8050425 ns/op   1394642 B/op   12035 allocs/op
BenchmarkFindingCSearchNoMatch/v7-indexed/N=1000      20     81315 ns/op      4024 B/op      82 allocs/op
BenchmarkFindingCSearchNoMatch/original-scan/N=5000   20  46036835 ns/op   7241691 B/op   60910 allocs/op
BenchmarkFindingCSearchNoMatch/v7-indexed/N=5000      20    142215 ns/op      4024 B/op      82 allocs/op
```

At N=5000 the original path is ~324× slower and allocates ~60,910 allocs/op
versus 82; the indexed path's time grows only mildly with N (81 µs → 142 µs) and
its allocations are flat.

---

## Reproduction matrix

| Finding | Reproduced on original | Reproduced on V7 baseline | Classification | Fixed on hardening branch |
|---|---|---|---|---|
| A — project scoping | yes (identical source) | yes | CONDITIONAL RISK / authz defect | yes (Phase 2) |
| B — multi-record transitions | yes (identical source) | yes | CONFIRMED RELIABILITY DEFECT | yes (Phase 4) |
| C — expensive search | yes (retained algorithm) | no (FTS5) | PERFORMANCE RISK / MITIGATED | characterized (Phase 6) |
