# Security

Beacon captures local agent session history and, optionally, serves reviewed project
memory to agents over MCP. The optimization work **hardened** the memory engine's
authorization, lifecycle, resource bounds, and provenance. This page summarizes it and
points to the full evidence.

> Scope: the review covers the **local** threat model (a process on the same machine
> that can write the configured store). No attack in the evidence is described as
> remote. No claim of invulnerability is made.

## What was confirmed in the original and fixed here

| Finding | Original | Optimized |
|---|---|---|
| Cross-project read via unscoped `get_memory` | exposed | **enforced** when a project allowlist is set |
| Caller-supplied project scope trusted | trusted | **authorized** (resolved + post-filtered) |
| Approval / supersession not atomic | half-applied on crash | **single transaction**, race-safe |
| Broad/non-selective search | O(N) scan | **bounded** (FTS5 + scan cap, overload error) |
| Oversized query / term / id | accepted | **rejected** |
| Per-call `*sql.DB` pool | leaked | **shared + Close** |
| MCP ignored `BEACON_MEMORY_DIR` | ignored | **honored** |
| Forged `source:verified` label | accepted as a tag | **cannot elevate** (opt-in Ed25519 attestation) |

Full matrix: [`reports/security/10_ORIGINAL_VS_V7_SECURITY_SCORECARD.md`](../../reports/security/10_ORIGINAL_VS_V7_SECURITY_SCORECARD.md).

## Default posture (important)

- **Single-user local is the default.** Cross-project read enforcement requires the
  operator to set a project allowlist. This is documented behavior, not a hidden gap.
- **Nothing is trusted by default.** `source:verified` / `source:untrusted` are
  attacker-writable text tags. Only an opt-in Ed25519 attestation verified against an
  operator-controlled `TrustRegistry` yields **Verified**.
- **localhost ≠ authenticated identity.** A local writer can forge tags or bodies; it
  cannot gain **Verified** status without a valid signature from a registered key.

## Provenance trust model

A memory is classified as:

- **Verified** — valid Ed25519 signature by a key registered in the trust registry, over
  `ContentDigest = SHA-256(id, project, kind, title, body)`, with the attestation's
  project matching.
- **Unverified** — no attestation, unknown/revoked key, or empty registry (fail-safe).
- **Invalid** — attestation present but failing (bad signature, tampered body, scope
  mismatch).

Verification is opt-in (`TrustContextOptions.Trust`); nil disables it, so the default
hot path is unchanged. Adversarial tests (forged label, tampered body, non-registered
key, revoked key, cross-project reuse, empty registry, selection integration) **all
pass**. See [`reports/v7-hardening/AUTHENTICATED_PROVENANCE_FIX.md`](../../reports/v7-hardening/AUTHENTICATED_PROVENANCE_FIX.md).

> **Deployment gate BLOCKED.** No protected key authority, OS keychain, or MCP client
> identity ships in this environment. The mechanism and tests are in place; production
> authenticated provenance requires an approval service holding a protected key whose
> public key is registered.

## Resource safety

- **Bounded search:** explicit overload error instead of an unbounded scan.
- **Request-shape limits:** oversized inputs are rejected.
- **Shared store:** one cached store per process; no per-call pool leak.
- **Write serialization:** the concurrency fix is durability-preserving (WAL,
  `synchronous` untouched).

## Still open (honest)

| Item | Status |
|---|---|
| Protected approval authority / MCP client identity | **BLOCKED** (not shipped) |
| Append-only security audit log | **NOT IMPLEMENTED** (mechanism scoped in P6 work) |
| Live-agent prompt-injection resistance | **NOT TESTED** (unit-level injection detection retained) |
| Multi-process writer correctness | **NOT TESTED** (cross-process work landed; soak not run) |
| 64-client / 1M scale | **NOT TESTED** |

## Reporting a vulnerability

See the repository [`SECURITY.md`](../../SECURITY.md) for how to report issues
privately. Do not open a public issue for a suspected vulnerability.

## Reproduce the security checks

```sh
cd cli/beacon
go test ./internal/learning/ -run 'TestContract_|TestSearchQuestion' -v -count=1
go test -race ./internal/learning/ -run 'Concurrency|Concurrent|Crash' -count=1
```

The provenance adversarial tests live in
`cli/beacon/internal/learning/provenance_test.go` (run with the package suite).
