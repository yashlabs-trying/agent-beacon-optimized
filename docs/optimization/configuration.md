# Configuration

The optimization work added a small number of environment variables and CLI flags. The
defaults are unchanged from upstream Beacon.

## Database placement

| Variable / flag | Default | Effect |
|---|---|---|
| `BEACON_MEMORY_DIR` | (unset) | Sets an explicit **local** directory for `memory.db`. Overrides the default location derived from the runtime log. Honored by the CLI and by `beacon mcp serve`. |

Default location (unchanged): user mode `$HOME/.beacon/endpoint/memory.db`; system mode
under the system log directory.

**Why it matters:** on synchronized or network mounts (WSL `drvfs`/`9p`/`/mnt/c`,
OneDrive, `cifs`, `nfs`, `fuse`, UNC) each SQLite commit fsyncs at ~1 s, giving
16-client write p99 ≈ 1.3–7.8 s. The engine detects these mounts via `CheckPlacement`
and prints a one-line warning; it **never moves the database silently**.

```sh
# Point the store at a local directory
export BEACON_MEMORY_DIR="$HOME/.local/share/beacon"
beacon memory list
```

Migration between locations is an explicit operator action (`MigrateDatabase`:
`VACUUM INTO` → `integrity_check` → row-count check → keep source, refuse overwrite).

Reference: [`reports/v7-hardening/DATABASE_PLACEMENT_FIX.md`](../../reports/v7-hardening/DATABASE_PLACEMENT_FIX.md).

## Retrieval

| Flag | Default | Effect |
|---|---|---|
| `memory list --question-aware --question "<q>"` | off | Routes to `SearchQuestion`: scoped lexical search, instruction-bearing note rejection, evidence selection, optional conditional semantic expansion, trust/conflict pass, whole-memory token budget + sufficiency signal. |
| `--semantic` | off | Allows the offline **hashed** embedder for conditional expansion. Does **not** enable ANN or a neural service. |

ANN (`UseANN`) and real neural retrieval remain **opt-in and disabled by default**.

Reference: [`reports/v7/V7_EVIDENCE_AND_VERDICT.md`](../../reports/v7/V7_EVIDENCE_AND_VERDICT.md)
(retained under `reports/`).

## Authorization

Project authorization is enforced when an allowlist is configured. In the default
single-user local deployment, cross-project enforcement is **not** active by design. See
[security](security.md) for the exact configuration and its limits.

## Provenance (opt-in)

Authenticated provenance activates only when a `TrustRegistry` is configured
(`TrustContextOptions.Trust`); nil disables it and the default hot path is unchanged.
Public keys are registered per `(approver, key_id)`; an empty registry trusts nothing.
See [security](security.md#provenance-trust-model).

## Concurrency

There are **no new tuning knobs**. The write-concurrency fix is internal (a
process-level mutex around the write transaction). SQLite `busy_timeout` (30 s), WAL,
and `synchronous` are unchanged.

## Reference environment

Benchmarks were taken on WSL2 Ubuntu, 8 vCPU, 7.6 GiB RAM, Go 1.27, `CGO_ENABLED=0`,
database on local ext4. See [benchmarks](benchmarks.md#environment).
