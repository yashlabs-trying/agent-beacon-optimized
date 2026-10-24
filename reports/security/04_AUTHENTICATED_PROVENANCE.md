# 04 — Authenticated Memory Provenance (Security Layer 2)

Status: **mechanism implemented and adversarially tested**; the end-to-end
approval gate is **BLOCKED** (no protected key authority / MCP caller identity in
this environment). This pass **verified** the existing mechanism; it did not need
to change it.

## Design (existing, verified)

Files: `cli/beacon/internal/learning/provenance.go`, `provenance_test.go`,
`context_select.go` (opt-in classification).

Separated concepts: memory content, claimed `source:` tag, authenticated source,
reviewer identity, approval status, signature validity, effective trust level.

- **Content binding.** `ContentDigest(m)` = SHA-256 over `{id, project, kind,
  title, body}`. Any content or scope change changes the digest.
- **Attestation.** Reserved tag `attest:v1:<base64(json)>` carrying
  `{v, approver, key_id, project, digest, sig}`, where `sig` is an Ed25519
  signature over the digest.
- **Trust registry.** `TrustRegistry` maps `(approver, key_id) -> public key`.
  Empty registry trusts nothing. `Add` rotates, `Remove` revokes.
- **Verification boundary.** `Authenticate` returns:
  - `Verified` — valid signature by a trusted key over the exact digest and
    matching project;
  - `Unverified` — no attestation / unknown key / no registry (fail-safe);
  - `Invalid` — attestation present but failing (bad signature, tampered
    content, scope mismatch).
- **No self-certification.** A `source:verified` tag alone is Unverified.
- **Cheap on the hot path.** One SHA-256 + one Ed25519 verify, only when a
  registry is configured; nil disables it (default).

## Adversarial tests (all PASS, this pass)

```
go test ./internal/learning/ -run 'TestProvenance|TestTrust' -count=1 -v
--- PASS: TestProvenanceForgedLabelIsNotTrusted
--- PASS: TestProvenanceValidAttestationVerified
--- PASS: TestProvenanceTamperedBodyInvalid
--- PASS: TestProvenanceInvalidSignatureRejected
--- PASS: TestProvenanceRevokedKeyNotTrusted
--- PASS: TestProvenanceCrossProjectRejected
--- PASS: TestProvenanceEmptyRegistryFailsSafe
```

| Attack / case | Expected | Result |
|---|---|---|
| Forged `source:verified` label | Unverified | PASS |
| Valid attestation | Verified | PASS |
| Body tampered after signing | Invalid | PASS |
| Signature by non-registered key | Invalid | PASS |
| Revoked key | not Verified | PASS |
| Attestation reused across projects (cross-project replay) | Invalid | PASS |
| Empty registry | not Verified (fail-safe) | PASS |

## What this pass did **not** do (BLOCKED / NOT TESTED)

- **No protected signing authority is wired.** No operator key file, OS
  keychain, or approval service signs memories. `SignAttestation` exists but is
  invoked only by tests. Until a protected private key and a key-to-reviewer
  mapping exist, `V7` does not claim authenticated provenance in production.
- **Concurrent revocation + retrieval and cache-after-revocation** were not
  exercised end-to-end (the mechanism has no cache of verified status; selection
  records authenticated IDs per call, so there is nothing to invalidate — this
  is by design, but it is reported as NOT TESTED rather than proven).
- **Recovery with missing key material** was not tested.

## Critical rule (kept)

If any untrusted client could invoke the signing authority to sign arbitrary
content, signatures would provide no approval security. No such interface is
exposed: there is no MCP/CLI mutation tool that signs. The gate stays BLOCKED
rather than shipped half-wired.

## Acceptance

| Requirement | Status |
|---|---|
| No elevation from self-asserted labels | PASS |
| Tamper / forgery / cross-project replay rejected | PASS |
| Revocation removes trust | PASS |
| Fail-safe default (empty registry) | PASS |
| End-to-end protected approval authority | **BLOCKED** |
