# 05 — Atomic and Race-Safe Memory Lifecycle (Security Layer 3)

Status: **implemented and tested** for the approval/rejection/supersession
transitions and crash atomicity.

## Objective

Make every memory-review transition durable and internally consistent: a crash
or a concurrent reviewer must never leave a half-applied review state.

## Root cause (Finding B)

`ApproveCandidateWithEdits` wrote the authoritative memory via `PutMemory` (one
transaction) and then the candidate via `PutCandidate` (a separate statement,
not serialized). A crash between them, or a concurrent reviewer, could leave an
approved memory whose candidate still read `candidate`. `SupersedeCandidate`
wrote the candidate and the superseded memory in separate operations.

## What changed

| File | Change |
|---|---|
| `cli/beacon/internal/learning/lifecycle.go` | New. `CommitCandidateTransition(candidate, expectState, *memory)` applies the candidate transition and, when a memory is supplied, the memory write in **one immediate transaction**. The candidate update is conditional on `state = expectState`; if the row is not in that state the whole transaction rolls back with `ErrCandidateNotReviewable`. Also extracted `writeMemoryTx`, `putCandidateTx`, `normalizeMemoryForWrite`, `normalizeCandidateForWrite`. |
| `cli/beacon/internal/learning/store.go` | `PutMemory` and `PutCandidate` refactored to use the shared helpers (behavior-preserving; `PutCandidate` now runs in a transaction). |
| `cli/beacon/internal/learning/candidate.go` | `ApproveCandidateWithEdits`, `RejectCandidate`, `SupersedeCandidate` now use `CommitCandidateTransition`. |

The derived FTS index, cache generation, and embeddings are still updated inside
the same transaction (via `writeMemoryTx`), and a derived-index failure still
never fails the authoritative write.

## Invariants enforced

- Rejected/superseded candidates cannot silently become approved (conditional
  state update).
- A candidate cannot be approved twice by concurrent reviewers (single winner).
- Supersession cannot leave conflicting active records (candidate + memory
  tombstone commit together).
- A memory and its review state cannot disagree after a successful transition.
- Failed transitions roll back (defer tx.Rollback + conditional check).
- Retries do not duplicate approved memories (deterministic IDs + upsert).
- Acknowledged mutations survive crash (WAL commit before return).

## Failure injection

`cli/beacon/internal/learning/security_lifecycle_test.go` spawns the test binary
as a subprocess and calls `os.Exit(2)` at each write stage via the existing
`faultHook` seam, then reopens the database and asserts the pair
(candidate state, memory rows) is never half-applied:

```
go test ./internal/learning/ -run 'TestApprovalCrashAtomicity|TestConcurrentApprovalSingleWinner|TestRejectedCandidateCannotBeApproved' -count=1 -v
--- PASS: TestApprovalCrashAtomicity (0.48s)
--- PASS: TestConcurrentApprovalSingleWinner (0.17s)
--- PASS: TestRejectedCandidateCannotBeApproved (0.15s)
```

Stages covered: `tx_begin_pre`, `tx_begin_post`, `memory_insert`, `fts_update`,
`generation_update`, `pre_commit` (must show neither the transition nor the
memory) and `post_commit` (must show both).

## Multi-process safety

`writeMu` serializes writers **within one process**. Cross-process safety relies
on SQLite `_txlock=immediate` + `busy_timeout=30000`, not on the mutex. A
dedicated multi-process writer test is planned (Phase 9); it is **NOT TESTED**
in this pass. This document does not claim the mutex solves cross-process
contention.

## Performance

The approval path now performs one transaction instead of two (one for the
memory, one statement for the candidate). This is equal or cheaper; it does not
hold the transaction open across model inference or remote calls (none occur in
this path). Write-latency targets are unaffected and are re-measured in the
performance report.

## Acceptance

| Requirement | Status |
|---|---|
| Atomic approval/rejection/supersession | PASS |
| Zero inconsistent final states under crash matrix | PASS (7 stages) |
| Concurrent reviewers single winner | PASS |
| Failed transition rolls back | PASS |
| Multi-process writer correctness | **NOT TESTED** (Phase 9) |
