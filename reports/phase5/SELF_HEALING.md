# Phase 5 — Self-Healing and Recovery

Two layers are reported separately: (1) the agent-level recovery task (A4) and (2) the
system-level crash/durability suites (reused, authoritative).

## 1. Agent-level recovery (A4, 20 tasks)

Tasks require using a fact that is only correct *after* a crash recovery (a stale
pre-crash note contradicts the durable post-recovery note).

| Arm | A | B | C | D | E |
|---|---:|---:|---:|---:|---:|
| A4 | 0/20 | 14/20 | 14/20 | 14/20 | 20/20 |

All memory arms 14/20; oracle 20/20. The 6 misses are model slips (the correct note was
retrieved). **No V6-vs-original difference.**

## 2. System-level crash/durability (reused suites, authoritative)

| Fault class | Test | Result |
|---|---|---|
| Crash at 7 transaction points | `TestPhase2CrashDurability` | PASS (0 acknowledged-write loss) |
| Crash during index rebuild / migration / supersession | `TestPhase2CrashDuring*` | PASS |
| Process kill mid-write | `TestPutMemoryProcessCrashRecovery` | PASS (6 sub-cases) |
| Reopen after interrupted startup | `TestPhase2ReopenAfterInterruptedStartup` | PASS |
| Corrupt row surfaced (not silent) | `TestPhase2CorruptRowSurfacedNotSilent` | PASS |
| Index rebuild after deletion | `TestPhase2IndexRebuildAfterDeletion` | PASS |

Phase 5 re-ran the full learning suite (**ok 47.6 s**) and the race subset (**ok 34.5 s**)
on the frozen Phase 5 commit; all crash/durability tests pass unchanged.

## 3. Honest distinctions

- **Recoverable derived-state damage ≠ corruption of authoritative data.** The suites
  verify acknowledged writes survive crashes; derived indices (FTS, embeddings, ANN) are
  rebuilt. No test demonstrated silent loss of authoritative memory.
- **A successful restart alone is not self-healing.** Recovery is asserted by data
  integrity checks, not by the process coming back up.
- Fabricated authoritative memory: not observed; corrupt rows are surfaced, not hidden.

## Verdict

Recovery: **PASS at the system level** (authoritative writes preserved; derived state
rebuilt); **EQUIVALENT at the agent level** (all memory arms 14/20). 1M-scale and
disk-exhaustion recovery: **NOT TESTED** (host limits).
