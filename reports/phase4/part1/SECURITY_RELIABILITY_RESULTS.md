# Phase 4A — Benchmark F: Security, Reliability, and Adversarial Results

**Gate: PASS (reused verified tests) — with two unchanged open limitations.**

Existing verified Go tests were re-run on the Phase 4A worktree rather than duplicated.
- Full package: `RAW_EVIDENCE/reliability_full_learning.txt`
- Targeted reliability/security: `RAW_EVIDENCE/reliability_targeted.txt`
- Race subset: `RAW_EVIDENCE/reliability_race.txt`
- HNSW: `RAW_EVIDENCE/reliability_hnsw.txt`
- Contradiction: `RAW_EVIDENCE/contradiction_locked_phase4a.txt`,
  `RAW_EVIDENCE/contradiction_phase4a.txt`

## 1. Executed on Phase 4A (WSL2, Go 1.27.3)

| Suite | Command | Result |
|---|---|---|
| Full learning package | `go test ./internal/learning/ -count=1` | **ok 48.9 s** |
| HNSW package | `go test ./internal/hnsw/ -count=1` | **ok 0.5 s** |
| Race (concurrency/contention subset) | `go test -race ./internal/learning/ -run 'Concurrency\|Concurrent\|PutMemoryWaits\|SearchConcurrent'` | **ok 28.5 s** |

## 2. Reliability / adversarial checks (all PASS)

| Requirement | Test | Result |
|---|---|---|
| Concurrent reads and writes | `TestPhase2ConcurrencyStress`, `TestConcurrentReadsWritesThenClose`, `TestSearchConcurrentReadersAndWriters` | PASS |
| SQLite integrity and recovery | `TestPhase2CrashDurability` (7 fault points), `TestPhase2ReopenAfterInterruptedStartup`, `TestPhase2CorruptRowSurfacedNotSilent` | PASS |
| Missing/damaged index recovery | `TestPhase2IndexRebuildAfterDeletion`, `TestPhase2CrashDuringIndexRebuild`, `TestPhase2CrashDuringMigration` | PASS |
| Incorrect project/version retrieval | `TestPhase2CrossScopeLeakageAllPaths`, `TestPhase2EmbeddingModelVersionIsolation` | PASS (0 leaks) |
| Contradiction FP/FN | `TestPhase2AdversarialContradictionMatrix`, `TestPhase3AContradictionLocked`, `TestPhase3BContradictionDev` | see §3 |
| Prompt injection via memory | `TestPhase2HostileMemoryContent`, `TestAdversarialMemoryRetrieval` | PASS |
| Unapproved evidence hidden | `TestPhase2UnapprovedEvidenceNotVisible` | PASS |
| Growing histories | `TestPhase2LargeMemory` | PASS |
| Write contention | `TestPutMemoryWaitsForBoundedWriteContention` (11.1 s) | PASS |
| Process crash recovery | `TestPutMemoryProcessCrashRecovery` (6 sub-cases) | PASS |
| Cache invalidation under writes/supersede | `TestPhase2CacheInvalidationUnderWrites`, `TestSearchCacheInvalidatedByWrite`, `TestSearchCacheInvalidatedBySupersede` | PASS |

## 3. Contradiction detection (locked 24-pair set)

| Mode | TP | FP | FN | TN | Precision | Recall |
|---|---:|---:|---:|---:|---:|---:|
| Default | 6 | 0 | 6 | 12 | 1.000 | **0.500** |
| Cross-kind (opt-in) | 13 | 0 | 0 | 10 | 1.000 | 1.000 |

Same-kind recall is 1.000; **all 6 misses are cross-kind**. There is one benign
cross-talk match (`c02` vs `c23`). Recall must **not** be compared across the 65-case
adversarial set (0.758) and this 24-pair locked set as if equivalent.

## 4. Adversarial balance (both directions)

- **Safety violations:** 0 — hostile memory content and unapproved evidence never bypass
  the trust boundary; injected instructions in retrieved memory are treated as data
  (`TestPhase2HostileMemoryContent`, and Benchmark A condition 7).
- **Excessive rejection of legitimate memories:** the contradiction detector's FP rate is
  0 on the locked set (no legitimate pair is falsely flagged); the cost is instead
  **false negatives** (cross-kind recall 0.50 default).

## 5. Unchanged open limitations (carried forward, not hidden)

| Limitation | Value | Status |
|---|---|---|
| 16-client write p99 | 744–795 ms (target < 250 ms) | **FAIL**; ≤8-client budget retained |
| Default contradiction recall | 0.500 locked | **FAIL** ≥ 0.90 gate |
| Token budgeting evidence loss | loses 3/109 findings | **FAIL**; opt-in |
| ANN build (100K M=64) | 102 s | within budget but costly |
| 1M ANN (M=64) | ≈83 min estimated | impractical |
| Neural retrieval | requires sidecar + model | opt-in |

Durability and transactional guarantees are preserved; no test was deleted or weakened.

## Conclusion

**Benchmark F: PASS** on all executed reliability/adversarial checks, with the 16-client
write tail and the default contradiction recall explicitly retained as open FAILs. No
safeguard was weakened to improve any number.
