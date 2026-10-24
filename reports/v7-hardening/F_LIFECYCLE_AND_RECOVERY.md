# Beacon V7 hardening — Phase F: Memory Lifecycle and Reliability

Status: **Durability and crash recovery verified (reused suites). Self-healing is
recovery-by-rebuild, not active repair. Feedback-driven learning is NOT implemented.**

## F1. Durability (reused, authoritative; re-run on this commit)

| Scenario | Test | Result |
|---|---|---|
| Interrupted transactions (7 fault points) | `TestPhase2CrashDurability` | PASS (0 acknowledged-write loss) |
| Concurrent updates | `TestPhase2ConcurrencyStress` (0 errors/busy/stale) | PASS |
| Crash during index rebuild/migration/supersession | `TestPhase2CrashDuring*` | PASS |
| Partial indexing / stale cache | `TestSearchCacheInvalidatedByWrite/Supersede` | PASS |
| Stale embeddings | `TestSearchQuestionBackfillsAndInvalidatesSemanticVectors` | PASS |
| Supersession | `TestPhase2CrashDuringSupersession`, `TestSelectContext_Supersession` | PASS |
| Deletes / index rebuild after deletion | `TestPhase2IndexRebuildAfterDeletion` | PASS |
| Restart recovery | `TestPhase2ReopenAfterInterruptedStartup`, `TestPutMemoryProcessCrashRecovery` | PASS |
| Migration interruptions | `TestPhase2CrashDuringMigration` | PASS |

Full learning suite re-run on this commit: **ok 58.0 s**; race subset: **ok 39.3 s**.

## F2. Crash injection

Injection at transaction, indexing, checkpoint, migration, and recovery boundaries is
covered by the Phase 2 fault-injection harness (deterministic child-process kill at
named points). All pass with durable state intact after restart.

## F3. Self-healing

**Honest scope:** the system recovers by **rebuilding derived state** (FTS index,
embeddings, ANN) from authoritative memory after a crash or index loss. This is
recovery, not active detection-and-repair of arbitrary inconsistency. The one active
repair path is index-unhealthy detection (`markIndexUnhealthy` → re-ensure schema /
rebuild).

| Measure | Value |
|---|---|
| Detection rate (index-unhealthy, corrupt row) | covered by tests (PASS) |
| Repair success (rebuild) | PASS |
| Recovery time | bounded; not separately benchmarked |
| Data preservation | 0 acknowledged-write loss |
| False repairs | not observed |

**Not claimed:** general self-healing; recovery from unrecoverable authoritative data loss.

## F4. Self-improvement — NOT IMPLEMENTED

**Finding:** the current architecture does **not** implement feedback-driven learning.
There is no mechanism that ingests task outcomes/corrections and adjusts future
retrieval or decisions. Supersession (a memory declaring `SupersededBy`, or a lexical
correction) records a *fact* change, but nothing observes agent success/failure and
updates behaviour. The earlier "post-feedback 56/56" pilot reflects fixture/harness
changes, not learned weights or learned policy.

**Minimal proposed design (not implemented):** an explicit feedback record
`{task_id, outcome, corrected_fact, source}` written through the normal approval path;
a periodic job that promotes corrected facts to higher trust and demotes facts
implicated in failures; a control comparison (with vs without feedback ingestion) to
measure repeated-error reduction. Until that exists, no self-improvement claim is made.

## Verdict

Durability: **PASS**. Crash recovery: **PASS**. Self-healing: **PARTIAL** (rebuild
recovery, not active repair). Self-improvement: **NOT IMPLEMENTED**.
