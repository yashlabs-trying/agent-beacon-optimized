# Beacon V7 — Phase 3: Authenticated Memory Provenance

Status: **Mechanism implemented and adversarially tested; the full authentication gate is
BLOCKED** (no protected key authority or MCP client identity exists in this environment).

Files: `cli/beacon/internal/learning/provenance.go`, `provenance_test.go`,
`context_select.go` (opt-in classification).

## 1. Threat model (declared)

- **Who can ingest/approve memory:** any local process that can write the store. The
  default deployment is single-user local.
- **What is trusted:** nothing by default. `source:verified` / `source:untrusted` are
  **attacker-writable text tags**, not proof.
- **localhost ≠ authenticated identity.** A local writer can forge any tag or body.
- **What needs cryptographic verification:** any claim of *elevated trust* ("this memory
  was approved by a specific approver"). Downgrades (untrusted) need no proof.
- **Key protection:** an Ed25519 signing key held by the approval authority; the public
  key is registered in an operator-controlled trust registry.

Note: in the current code `source:verified` does **not** itself grant elevation — the
trust layer only *downgrades* memories carrying untrusted markers. The provenance layer
makes the positive claim ("approved by X") cryptographically verifiable.

## 2. Design (minimal, sound)

- **Content binding.** `ContentDigest(m)` = SHA-256 over `{id, project, kind, title, body}`.
  Any content or scope change changes the digest.
- **Attestation.** A reserved tag `attest:v1:<base64(json)>` carrying
  `{v, approver, key_id, project, digest, sig}` where `sig` is an Ed25519 signature over
  the digest. Reserved namespace is distinct from informational `source:` tags.
- **Trust registry.** `TrustRegistry` maps `(approver, key_id) → ed25519.PublicKey`.
  Empty registry trusts nothing (safe default). `Add`/`Remove` support rotation/revocation.
- **Verification boundary.** `Authenticate(m, registry)` returns:
  - **Verified** — valid signature by a trusted key over the exact digest and matching
    project.
  - **Unverified** — no attestation, malformed, or signer not trusted/active (fail-safe).
  - **Invalid** — attestation present but failing (bad signature, tampered content,
    scope mismatch).
- **Fail-safe.** Absence of verification never grants trust; unknown/revoked keys are
  Unverified; the default registry is empty so every legacy record is Unverified.
- **No self-certification.** A `source:verified` tag alone yields Unverified.
- **Cheap on the hot path.** Verification is one SHA-256 + one Ed25519 verify, only when
  a registry is configured (`TrustContextOptions.Trust`); nil disables it (default).
- **Selection integration.** When a registry is set, `SelectTrustContext` records
  `AuthenticatedIDs` (valid attestations) and `ProvenanceInvalidIDs`; it never changes
  which memories are selected, and injection defense is unaffected.

## 3. Adversarial tests — all PASS

| Attack / case | Expected | Result |
|---|---|---|
| Forged `source:verified` label | Unverified | PASS |
| Valid attestation | Verified | PASS |
| Body tampered after signing | Invalid | PASS |
| Signature by non-registered key | Invalid | PASS |
| Revoked key | not Verified | PASS |
| Attestation reused across projects | Invalid | PASS |
| Empty registry | not Verified (fail-safe) | PASS |
| `SelectTrustContext` authenticated IDs | only the signed memory | PASS |

## 4. Performance

Verification runs only when a registry is configured. Cost is one SHA-256 + one Ed25519
verify per survivor (sub-microsecond to low-microsecond). With no registry (default) the
hot path is unchanged; the Phase 2 allocation result (875 allocs/op) is unaffected
(full suite + race pass).

## 5. Acceptance

| Requirement | Status |
|---|---|
| Enforced verified vs unverified distinction | **PASS** (mechanism) |
| No elevation via self-asserted labels (tested threat model) | **PASS** |
| No unauthorized trust promotion | **PASS** |
| No material retrieval regression | **PASS** (opt-in; unchanged when off) |
| Security evidence with reproduction tests | **PASS** |
| Full authentication gate (protected key authority + MCP identity) | **BLOCKED** |

## 6. Blocker (explicit)

There is no protected signing authority, OS keychain integration, or MCP client identity
in this repository/environment. The schema, verification boundary, fail-safe behavior,
rotation/revocation, and adversarial tests are implemented and passing, but **V7 does not
claim authenticated provenance in production** until (a) an approval service holds a
protected private key, (b) its public key is provisioned into the registry, and
(c) MCP/CLI approval paths are wired to sign. Until then the gate is **BLOCKED**, and
legacy/unsigned records remain **Unverified** by design.

## 7. Verdict

Mechanism: **PASS**. End-to-end authenticated deployment: **BLOCKED** (documented).
