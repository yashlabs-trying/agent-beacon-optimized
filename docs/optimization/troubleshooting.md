# Troubleshooting

Common issues when running or reproducing the optimization work.

## "My benchmarks are ~12× slower than the numbers"

**Cause:** the database is on a synchronized/network filesystem (WSL `/mnt/c`,
OneDrive, `cifs`, `nfs`, `fuse`, UNC). Each SQLite commit fsyncs at ~1 s.

**Check:** run a `memory list` and look for the placement warning, or inspect where the
store resolves.

**Fix:**

```sh
export BEACON_MEMORY_DIR="$HOME/.local/share/beacon"   # a local ext4 directory
```

Or benchmark with `TMPDIR=/tmp` (not a Windows mount). Migration between locations is
explicit (`MigrateDatabase`: `VACUUM INTO` + integrity + row-count, keeps the source).
See [configuration](configuration.md#database-placement).

## "`go test ./...` doesn't finish / is very slow"

The full CLI module suite is large and slow on a Windows-mounted/OneDrive checkout.
Run the focused modules:

```sh
cd cli/beacon
go test ./internal/learning/ -count=1
go test ./internal/hnsw/ -count=1
```

This is a documented coverage gap ([limitations](limitations.md)), not a failure.

## "`gofmt -l` lists many files"

Pre-existing Windows CRLF checkout artifact. It appears on unmodified files too; it is
not formatting drift introduced by the optimization. Use an LF worktree (e.g. WSL) for
formatting checks.

## "The 16-client writer test shows a huge p99"

Check the filesystem first (above). On local ext4 it should be well under 250 ms. If it
is still high, confirm you are on the optimized commit `db341fbc` and that the writer
harness is the one documented in [benchmarks](benchmarks.md), not a modified copy.

## "Cross-project memory access isn't blocked"

By design in the **default single-user local** mode. Enforcement requires a configured
project allowlist. See [security](security.md#default-posture).

## "`source:verified` shows as not trusted"

Correct. `source:verified` is an informational, attacker-writable tag and never grants
elevated trust. Only an opt-in Ed25519 attestation verified against a configured
`TrustRegistry` yields **Verified**. With no registry (the default), everything is
**Unverified** — fail-safe. See [security](security.md#provenance-trust-model).

## "Which numbers can I quote?"

Only the **reproducible** rows in [performance](performance.md). Anything marked
historical (recall@5 figures, ANN 100K, the 56-task pilot, the 300-task V6 result) must
carry that label. See [limitations](limitations.md#how-to-avoid-misreading-this-repo).

## "Build fails with a missing `hooks.bin`"

The CLI embeds a hooks binary. A fresh clone needs a placeholder when building the
`embedded` package directly:

```sh
cd cli/beacon
[ -f internal/embedded/hooks.bin ] || echo PLACEHOLDER > internal/embedded/hooks.bin
```

The normal `make build` path handles this (`ensure-placeholder`).

## Still stuck?

Open a [bug report](../../.github/ISSUE_TEMPLATE/bug_report.md) with your OS, Go
version, the exact command, and the output.
