# Beacon V7 — Final Report: Three Remaining Problems

## 1. Git provenance

- Repository: `C:\\Users\\<user>\\work\rescue\beacon` (git common dir).
- Worktree: `…\beacon-v7-hardening`; branch `codex/beacon-v7-hardening-3`.
- Base (reported stabilized HEAD): `0cdbeb6af8b7786e302231cc18c98a25260637da`.
- **Final HEAD: `500e2a0e86d60586b086a47f70587688d3d3f8e7`.**
- Local commits this session: `2bcc2560` (Phase 1), `90b21107` (Phase 2), `500e2a0e` (Phase 3).
- Original Beacon `99c70908` and stabilized V7 `0cdbeb6a` preserved; no push/PR/deploy.

## 2. Database placement — baseline and final

- **Baseline:** database location derived solely from the runtime log; a DB under
  `/mnt/c` (OneDrive/drvfs) is ~12× slower (16-client write p99 ~7.76 s; ~1 s/commit).
- **Final:** `BEACON_MEMORY_DIR` sets an explicit local database directory; `CheckPlacement`
  detects drvfs/9p/cifs/nfs/fuse and UNC mounts and warns; `MigrateDatabase` copies with
  `VACUUM INTO` (WAL-consistent) + `integrity_check` + row-count verification, refuses to
  overwrite, keeps the source. Default path unchanged (`$HOME/.beacon/endpoint/memory.db`).
- Placement: **PASS** (detection, override, safe migration, 5 tests).

## 3. 16-client write p99

| | p50 | p95 | p99 | errors/busy |
|---|---:|---:|---:|---:|
| Baseline (ext4, pre-fix) | 2.9 ms | 133.9 ms | **634 ms** | 0 / 0 |
| **Final (ext4)** | 24.4 ms | 44.7 ms | **53.4 ms** | 0 / 0 |

Final matrix: 8 clients p99 28.8 ms; 16 → 53.4 ms; 32 → 93.6 ms. **Gate ≤250 ms: PASS.**
OneDrive baseline (~7.76 s) remains a filesystem limit, now detected/warned.

## 4. Retrieval allocations (warm question-aware, 1,000-memory store)

| Metric | Before | After | Change |
|---|---:|---:|---:|
| Allocs/op | 2,007 | **875** | **−56.4%** |
| Bytes/op | 208,054 | 130,495 | −37.3% |

**Target ≥50% reduction: PASS.** No correctness/trust/cache regression.

## 5. Retrieval latency

- `ListMemories` (Limit 8): ~58–78 µs, 64 allocs (unchanged).
- `SearchQuestion` warm: mean ~0.82–1.30 ms/op (was ~1.3–1.6 ms) on this host.
- Cold healthy store: V7 20–36 ms vs original 20–65 ms (parity); first-ever query pays a
  one-time trigram index build.
- **p95/p99 of `SearchQuestion` were NOT separately measured** (host is I/O-noisy);
  allocations are the stable, reported metric. `ListMemories`/`SearchQuestion` have
  different semantics and are not presented as a direct ratio.

## 6. Cache correctness

Existing bounded (32 MiB LRU), generation-invalidated, project-scoped cache. All cache
tests pass: `TestSearchCacheInvalidatedByWrite`, `…BySupersede`,
`TestSearchConcurrentReadersAndWriters`, `TestSearchConcurrentSupersedeAndRead`,
`TestPhase2CacheInvalidationUnderWrites`, `TestSearchCrossStoreVisibility`. **PASS.**

## 7. Exact provenance trust model

- A memory is **Verified** only with a valid Ed25519 signature by a key registered in the
  operator-controlled `TrustRegistry`, over `ContentDigest = SHA-256(id, project, kind,
  title, body)`, with the attestation's project matching the memory's project.
- **Unverified** = no attestation / unknown or revoked key / empty registry (fail-safe).
- **Invalid** = present but failing (bad signature, tampered content, scope mismatch).
- `source:verified` and other informational tags are **never** proof.
- Verification is opt-in (`TrustContextOptions.Trust`); nil = off (default), so the hot
  path is unchanged. When on, `SelectTrustContext` records `AuthenticatedIDs` and
  `ProvenanceInvalidIDs` without changing selection or injection defense.

## 8. Adversarial tests

Passing (8): forged `source:verified` label → Unverified; valid attestation → Verified;
tampered body → Invalid; non-registered-key signature → Invalid; revoked key → not
Verified; cross-project reuse → Invalid; empty registry → not Verified; selection
integration → only the signed memory authenticated.

Failing: none.

## 9. Crash durability

Crash at 7 transaction points, crash during index rebuild/migration/supersession,
process-kill recovery, reopen-after-interrupted-startup, corrupt-row surfacing — all
PASS. Phase 1 (write mutex) and Phase 3 (provenance, read-only) preserve every guarantee.

## 10. Full test-suite results

| Check | Result |
|---|---|
| `go test ./internal/learning/` | **ok 38.9 s** |
| `go test -race` (concurrency/crash subset) | **ok 32.3 s** |
| `go test ./internal/hnsw/` | ok |
| `go vet ./internal/learning/` | clean |
| `go build ./...` | pass |
| Question-aware tests (12) + contract tests (12) | PASS |
| Provenance tests (8) | PASS |
| Placement/migration tests (5) | PASS |

Not run: full repository-wide CLI suite, MCP/OpenCode integration, cross-platform,
1M scale, 24–72 h soak. `gofmt -l` lists many files due to the Windows CRLF checkout
(pre-existing environmental artifact).

## 11. Binary size and peak RAM

- Binary: **47,494,546 bytes (~45.3 MiB)**.
- Peak RSS during 16/32-client writer runs: **~27–28 MB**.

## 12. Remaining blockers

1. **Authenticated provenance gate BLOCKED:** no protected key authority, OS keychain, or
   MCP client identity in this environment. Mechanism + tests are in place; deployment
   needs an approval service holding a protected key and its public key registered.
2. **64-client and 1M/10M scale:** NOT TESTED (hardware).
3. **`SearchQuestion` p95/p99:** NOT separately measured.
4. **MCP/OpenCode integration and full CLI suite:** NOT TESTED in this pass.
5. **Semantic-vs-lexical ablation:** NOT TESTED (semantic stays optional/off).

## 13. Release-readiness verdict

| Gate | Verdict |
|---|---|
| Safe local DB placement + migration | **PASS** |
| 16-client write p99 ≤250 ms | **PASS** (53 ms) |
| Allocation reduction ≥50% | **PASS** (56.4%) |
| No correctness/trust/cache regression | **PASS** |
| Authenticated provenance | **PARTIAL / BLOCKED** (mechanism PASS, deployment BLOCKED) |
| Crash durability | **PASS** |
| Full integration coverage | **PARTIAL** (MCP/full CLI/cross-platform not run) |
| Scale (1M, 64 clients) | **NOT TESTED** |

**V7 is materially improved and safe on a local filesystem, but not yet fully
production-ready:** the three targeted problems are fixed (placement, allocations, and a
provenance mechanism), with the provenance *deployment* gate explicitly blocked pending a
protected key authority, and integration/scale coverage still partial. The independent
500-task holdout was **not opened, generated, or run**.
