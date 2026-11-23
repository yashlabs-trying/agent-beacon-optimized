# Beacon V7 — Phase 1: Safe Local Database Placement

Status: **Implemented and tested.** Placement is now configurable, unsuitable
(synchronized/network) filesystems are detected and warned about, and a transactionally
consistent migration is provided. The database is never moved silently.

Branch: `codex/beacon-v7-hardening-3` (from `0cdbeb6a`).
Files: `cli/beacon/internal/learning/placement.go`, `placement_test.go`,
`cli/beacon/cmd/memory.go`.

## 1. Effective database path (CLI / MCP / background)

- CLI: `memoryStore()` → `learning.DatabasePath(memoryLogPath())`.
- Default (unchanged): the store lives next to the runtime log —
  user mode `$HOME/.beacon/endpoint/memory.db`; system mode under the system log dir.
  On WSL, `$HOME` is the local ext4 home, so the default is already local.
- **New override:** `BEACON_MEMORY_DIR` sets an explicit database directory, so the
  source worktree, the runtime log, and the database location are independent.
- MCP/background resolve through the same `DatabasePath`, so there is exactly one
  authoritative location per configuration.

## 2. Filesystem suitability detection

`CheckPlacement(path)` inspects the mount holding the database and returns
`{Path, Filesystem, Synchronized, Warning}`:

- Linux: parses `/proc/mounts` for the longest matching mount point and flags
  `drvfs`, `9p`, `cifs/smb3`, `nfs`, `vboxsf`, and `fuse.*` (rclone/sshfs) as
  synchronized/network. Fallback: any `/mnt/<drive>` path is treated as a WSL
  Windows-drive mount.
- Windows: UNC (`\\host\share`) paths are flagged.

The CLI prints a clear one-line warning to stderr (`memory list`) when the resolved
database is on such a filesystem. It never moves the database.

## 3. Safe migration (`MigrateDatabase(src, dst)`)

- Uses **`VACUUM INTO`**, which produces a transactionally consistent snapshot that
  includes committed WAL contents — **not** a naive file copy that would ignore `-wal`.
- Refuses to overwrite an existing destination.
- After the copy: runs `PRAGMA integrity_check` and compares the `memories` row count;
  success is reported only if integrity is `ok` and the counts match.
- On any failure it removes the partial destination and leaves the **source untouched**.
- The source is never deleted; the operator switches to the destination explicitly
  (e.g. by setting `BEACON_MEMORY_DIR`).

## 4. Tests (`placement_test.go`) — all PASS

| Test | Result |
|---|---|
| `TestDatabasePathHonorsMemoryDirEnv` | PASS |
| `TestCheckPlacementFlagsSynchronizedMount` (`/mnt/c` → synchronized) | PASS |
| `TestCheckPlacementAllowsLocalDir` (temp dir) | PASS |
| `TestMigrateDatabaseRoundTrip` (25 memories; source preserved; destination readable) | PASS |
| `TestMigrateRefusesExistingDestination` | PASS |

## 5. Performance re-verification (this host, ext4, mixed, 8 s)

| Clients | write p99 | errors/busy |
|---:|---:|---:|
| 16 | **54 ms** | 0 / 0 |

The write-serialization fix is intact (target ≤250 ms). OneDrive remains unusable
(historical ~1 s/commit, 16-client p99 ~7.8 s) — now detected and warned instead of
silently tolerated.

## 6. Acceptance

| Requirement | Status |
|---|---|
| Configurable local DB directory | PASS (`BEACON_MEMORY_DIR`) |
| Source worktree separate from runtime DB | PASS (default `$HOME/.beacon`, override) |
| Warn on synchronized/network mounts | PASS |
| Preserve DB compatibility | PASS (default path unchanged; same file format) |
| Never silently relocate/delete | PASS (explicit migration only) |
| Transactional migration + integrity + counts + keep source | PASS |
| Permissions/read-write failures | PASS (mkdir 0700; errors surfaced) |
| One authoritative location | PASS (single resolver) |
| 16-client p99 < 250 ms | PASS (54 ms) |

## 7. Remaining

- Automatic migration-on-startup is intentionally **not** implemented (would violate
  "never silently relocate"); migration is an explicit operator action.
- `BACKUP`/restore of a live store under concurrent writers is exercised by the
  `VACUUM INTO` snapshot semantics and the durability suite; a dedicated
  concurrent-backup soak is **NOT TESTED**.
