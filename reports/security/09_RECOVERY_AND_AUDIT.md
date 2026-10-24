# 09 — Recovery and Audit Integrity (Security Layer 7)

Status: **recovery verified by existing tests plus the new atomic-lifecycle
crash matrix; security audit log NOT IMPLEMENTED.**

## Recovery

Verified (assertions pass; Windows TempDir cleanup artifact aside):

| Scenario | Test |
|---|---|
| Crash during a memory write, all stages | `crash_recovery_test.go` `TestPutMemoryProcessCrashRecovery`, `phase2_crash_injection_test.go` (Unix) |
| Crash during approval / supersession (atomicity) | `security_lifecycle_test.go` `TestApprovalCrashAtomicity` (7 stages) |
| Database reopen / WAL recovery | `phase2_integration_test.go`, `busy_lock_test.go` |
| Index drop → rebuild | `TestSearchRecoversAfterIndexDrop`, `TestSearchIndexRebuildIsIdempotent` |
| Legacy schema upgrade | `phase2_integration_test.go` `TestPhase2LegacyUpgrade*` |
| Multi-store lifecycle / pool close | `phase3c_pool_test.go` |

The new `TestApprovalCrashAtomicity` (this pass) kills the process at
`tx_begin_pre`, `tx_begin_post`, `memory_insert`, `fts_update`,
`generation_update`, `pre_commit`, and `post_commit`, then reopens the database
and asserts the pair (candidate state, memory row) is never half-applied.

Recovery does not trust corrupted/unverifiable security state: the derived index
is marked unhealthy on error and rebuilt; provenance defaults to Unverified when
the registry is empty or a key is unknown.

**NOT TESTED:** backup/restore end-to-end, incomplete-migration recovery, and
key-revocation-after-restart.

## Audit

**NOT IMPLEMENTED.** There is no dedicated security audit record for approvals,
rejections, supersessions, trust promotions, key rotation/revocation,
authorization denials, or failed integrity verification.

The review trail that does exist: `candidates.candidate_json` retains review
state, `review_reason`, `approved_at`/`rejected_at`/`superseded_at`, and the
`updated_at`/`generation` metadata. This is **not** a tamper-evident audit log.

**No tamper-evidence is claimed.** A local hash chain without an independent
trust anchor is not proof against an attacker who controls the whole database;
none is implemented, and none is claimed.

## Acceptance

| Requirement | Status |
|---|---|
| Recovers consistently from tested failures | PASS |
| Acknowledged writes survive tested crashes | PASS |
| Index/cache recovery | PASS |
| Does not auto-trust unverifiable security state | PASS |
| Minimal security audit record | **NOT IMPLEMENTED** |
| Tamper-evident audit | **NOT IMPLEMENTED** (not claimed) |
