# 08 — Cache, Filesystem, and Sensitive-Data Protection (Security Layer 6)

Status: **cache isolation verified by existing tests; one filesystem-consistency
bug fixed; secret-redaction NOT TESTED.**

## Cache security

The search cache is keyed by store path, query, and the durable index
`generation`; a write bumps the generation in the same transaction, so a cached
result under an older generation is never served. Authorization is applied
**before** and **after** the cache boundary: `search_memory`/`get_memory_context`
authorize the project before querying, and `authorizedMemories` filters rows
after retrieval, so a cached row cannot bypass project authorization.

Verified by existing tests (this pass, assertions pass; Windows TempDir cleanup
artifact aside):

| Test | Asserts |
|---|---|
| `TestSearchCacheInvalidatedByWrite` | cache invalidated on write |
| `TestSearchCacheInvalidatedBySupersede` | cache invalidated on supersession |
| `TestPhase2CacheInvalidationUnderWrites` | invalidation under concurrent writes |
| `TestPhase2CrossScopeLeakageAllPaths` | no cross-scope leakage on any path |
| `TestSearchConcurrentReadersAndWriters` | readers/writers consistency |

Added this pass: `authorizedMemories` runs on every search/context result, so
even a stale or mis-scoped cached row is dropped before it leaves the server.

**NOT TESTED:** cross-process cache invalidation (the cache is per-process; the
generation check reads the DB, which is the cross-process guard, but no
multi-process cache test was run).

## Filesystem protection

Existing: `BEACON_MEMORY_DIR` (`placement.go`) lets the operator place the
database on a local filesystem; `CheckPlacement` warns on
synchronized/network mounts; migration uses `VACUUM INTO` with integrity and
row-count verification and never deletes the source before the destination is
verified; the store directory is created `0700`.

**Bug fixed this pass:** the MCP server computed its store path with
`learning.PathForRuntimeLog(logPath)`, ignoring `BEACON_MEMORY_DIR`. It now uses
`learning.DatabasePath(logPath)`, so the documented placement override is
honored consistently with the CLI. (Default behavior is unchanged when the env
var is unset, which is why tests still pass.)

**NOT TESTED:** path traversal via `project` arguments (mitigated by
`ProjectIDForPath`, which never touches the filesystem for a request-supplied
path — but no dedicated traversal fuzz was run), symlink handling, file
ownership checks, atomic file replacement of the DB, and Agent Skill install
paths.

## Sensitive information

Existing `asymptoteobserve.CleanString` truncates memory bodies in MCP
summaries (1200 chars). The store does not redact secrets by content.

**NOT TESTED:** redaction/retention policies on synthetic secrets in logs, MCP
responses, debug reports, cached memory, config, or skill output. This report
does **not** claim that redaction identifies every possible secret.

## Acceptance

| Requirement | Status |
|---|---|
| Cached results cannot bypass authorization | PASS (pre + post filter) |
| Cached results cannot bypass write/supersession | PASS (existing tests) |
| Placement override honored by all surfaces | PASS (fixed) |
| Migrations do not lose committed state | PASS (existing `VACUUM INTO` verify) |
| Cross-process cache invalidation | **NOT TESTED** |
| Synthetic-secret redaction | **NOT TESTED** |
| Path traversal / symlink / ownership | **NOT TESTED** |
