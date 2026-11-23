# 01 — Baseline Threat Model

Status: **established**. This is the declared threat model the hardening work is
measured against. Anything not listed here is out of scope and is not claimed as
protected.

## Environment (as executed)

| Item | Value |
|---|---|
| OS | Microsoft Windows 11 Home Single Language 10.0.26200 (build 26200) |
| Arch | AMD64 |
| CPU | 11th Gen Intel(R) Core(TM) i5-1155G7 @ 2.50GHz (4 cores / 8 threads) |
| RAM | 15.7 GB |
| Go | go1.27.0 windows/amd64 |
| Git | 2.55.0.windows.3 |
| Worktree | `C:\\Users\\<user>\\work\rescue\beacon-v7-hardening` |
| Hardening branch | `codex/beacon-v7-security-hardening` |
| Baseline tag | `baseline-v7-7c17c5fd` (V7) |
| Frozen original tag | `baseline-original-99c70908` |

Authoritative build/test platform is Linux in upstream CI. This pass runs on
Windows; see "Environment limitations" below.

## Verified commits

| Role | Commit | Verified locally |
|---|---|---|
| Frozen original Beacon | `99c709081ad49da1c5ade205929b61b19360ed9b` | yes (`cat-file`, worktree) |
| Earlier stabilized V7 | `0cdbeb6af8b7786e302231cc18c98a25260637da` | yes; ancestor of HEAD |
| Latest hardened V7 (baseline) | `7c17c5fd1503769e0fc147c0f9d3c551ee89c539` | yes; == HEAD at start |
| V7 branch | `codex/beacon-v7-hardening-3` | yes |

`99c70908` and `0cdbeb6a` are both ancestors of `7c17c5fd`.

## Deployment modes

1. **stdio MCP (default).** The MCP server runs as a child process of the local
   agent harness, under the same OS user. `beacon mcp serve --transport stdio`.
   The trust boundary is the **operating-system process/user boundary**: any
   process that can speak to this server already runs as the store owner and can
   read `memory.db` directly.
2. **HTTP MCP (`--transport http`, loopback only).** Bound to
   `127.0.0.1:8766` by default. `dashboard.ValidateLoopbackAddr` rejects
   non-loopback addresses and `dashboard.RequireLoopbackHost` rejects
   non-loopback `Host` headers (DNS-rebinding defense). The trust boundary is
   still the OS user boundary: loopback + same user.

## Assets

- Approved memory records (`memories` table; `memory.db`).
- Candidate and evaluation review records (`candidates`, `evaluations`).
- The derived FTS5/embedding index and cache generation.
- Provenance/attestation tags and (proposed) approval signing keys.

## Adversaries

| Adversary | In scope? | Rationale |
|---|---|---|
| Malicious/compromised memory **content** (poisoning) | yes | retrieved memory can carry instructions |
| A second local agent/process running as the **same OS user** that should not read another project's memory | yes (mechanism) | the reported Finding A concern; becomes an authorization failure when scopes are meant to be isolated |
| A local process running as a **different OS user** | no (mitigated by OS) | store dir is created `0700`; DB file is the owner's |
| A **remote** network attacker | no | HTTP is loopback-only; no remote surface is exposed |
| A caller who can invoke the signing authority to sign arbitrary content | yes | would defeat approval security; must not be shipped |

## Trust boundaries and rules

- Authority for project access is **server-side**, never derived from MCP
  request arguments.
- `localhost` is **not** authentication. A local writer can forge any memory
  text or `source:` tag.
- Absence of verification never grants trust (fail-safe).
- Retrieved memory is **data**, not a trusted control instruction.

## Security objectives (tested)

1. No cross-project memory read via search, context, or unscoped ID lookup when
   an authorization policy is configured.
2. No trust elevation from self-asserted labels; provenance is cryptographic.
3. Every multi-record review transition is atomic and crash-consistent.
4. Retrieved content cannot gain instruction authority.
5. Expensive queries cannot monopolize the engine (bounded work).
6. Cached results cannot bypass authorization, deletion, supersession, or
   revocation.
7. Security-relevant transitions are auditable; recovery does not trust corrupt
   state.

## Explicitly BLOCKED guarantees

- **Authenticated provenance end-to-end.** No protected signing authority or
  OS keychain integration exists in this environment, so the approval signing
  gate cannot be wired to a protected private key. The verification mechanism
  is implemented and tested; the deployment claim is BLOCKED.
- **Authenticated MCP caller identity.** No authentication provider exists. In
  the local deployment the caller identity is the OS process boundary. Shared
  multi-tenant operation is therefore not offered as a guarantee; it is gated
  behind an explicit project allowlist.

## Environment limitations (reported, not hidden)

- **Windows Application Control** intermittently blocks freshly built test
  executables (`go test` writes to `%TEMP%\go-build*`). Workaround used:
  `go test ./pkg/ -count=1` directly (succeeds) or `go test -c -o <dir>` and run
  the binary. Where a command was blocked, it is reported as NOT TESTED.
- **Windows `t.TempDir` cleanup**: tests that leave a SQLite handle open fail
  cleanup with `unlinkat ... The process cannot access the file`. This is a
  harness artifact (Linux unlinks open files). Two Unix-only test files
  (`phase2_crash_injection_test.go`, `phase2_hnsw_bench_test.go`) were given
  `//go:build !windows` constraints and their platform-neutral helpers moved to
  `crash_helpers_test.go` so the suite compiles on Windows.
- The independent 500-task intelligence holdout was **not** accessed, tuned
  against, or executed.
