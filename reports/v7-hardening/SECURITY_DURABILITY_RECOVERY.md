# Beacon V7 hardening — Phase 6: Security, Durability, Recovery

Status: **Core guarantees hold (reused, authoritative). Authenticated provenance and
cross-tenant authorization are NOT implemented.**

## 1. Durability / memory reliability (reused suites, re-run on this commit)

| Requirement | Test | Result |
|---|---|---|
| No lost acknowledged writes | `TestPhase2CrashDurability` (7 fault points) | PASS |
| No duplicate mutations under retry | `TestPutMemoryProcessCrashRecovery` (6 sub-cases) | PASS |
| Atomic updates | crash-during-transaction tests | PASS |
| Correct deletion | `TestPhase2IndexRebuildAfterDeletion` | PASS |
| Correct supersession | `TestPhase2CrashDuringSupersession` | PASS |
| No stale FTS entries | index rebuild + generation tests | PASS |
| No stale cache entries | `TestSearchCacheInvalidatedByWrite/Supersede` | PASS |
| Safe migration | `TestPhase2CrashDuringMigration`, `TestPhase2UpgradeFromOriginalStorage` | PASS |
| Interrupted-write recovery | `TestPhase2ReopenAfterInterruptedStartup` | PASS |
| Consistent state after crash | crash suite | PASS |
| Backup/restore | **NOT TESTED** (no backup tooling in this pass) |

Phase 1 added a write mutex; it preserves all of the above (each write still commits
under WAL with `synchronous` untouched). The durability and race suites pass after the
change.

## 2. Security

| Requirement | Status |
|---|---|
| Prompt-injection resistance | PASS (tested set; `SearchQuestion` rejects instruction-bearing facts; SelectTrustContext defenses) |
| Memory/data vs instruction separation | PASS |
| Project isolation | PASS (`TestPhase2CrossScopeLeakageAllPaths`, question scope test) |
| Cache isolation | PASS (project in key; generation-invalidated) |
| Untrusted source handling | PASS |
| Tenant authorization boundaries | **NOT IMPLEMENTED / NOT TESTED** |
| Unauthorized updates/deletes | **NOT TESTED** (no auth layer) |
| MCP permission enforcement | **NOT TESTED** |

## 3. Provenance

`source:verified` / `source:untrusted` are plain tags, **not cryptographic
authentication**. Whether the product's trust model *requires* authenticated provenance
is a product decision; if it does, the smallest mechanism is Ed25519 ingestion
identities with signed memory records and a trusted-key registry (design in
`E_SECURITY_AND_PROVENANCE.md`). No key-management authority was available in this
environment, so this is documented as a **security blocker**, not pretended solved.

## 4. Recovery

- Index rebuild on `index_ready=0` or detected index failure: PASS.
- Restart correctness: PASS (`TestPhase2ReopenAfterInterruptedStartup`).
- Corrupted derived-state detection: `TestPhase2CorruptRowSurfacedNotSilent` PASS.
- This is **rebuild recovery**, not autonomous self-healing; described as such.
- Feedback-driven self-learning: **not implemented** (out of scope for stabilization).

## 5. Verdict

Durability: **PASS**. Recovery (rebuild): **PASS**. Injection/project/cache isolation:
**PASS (tested set)**. Authenticated provenance and cross-tenant authorization:
**NOT IMPLEMENTED** (blocker documented).
