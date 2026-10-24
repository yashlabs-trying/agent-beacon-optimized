# Beacon V7 hardening — Phase E: Security and Provenance

Status: **Partially tested. Injection defenses hold on the tested set; authenticated
provenance is NOT implemented.**

## E1. Prompt injection (existing coverage, re-confirmed)

V7's `SearchQuestion` rejects instruction-bearing evidence before it can become an
answer (`TestSearchQuestionDropsInstructionBearingFactEvenWhenVerified`), in addition to
the `SelectTrustContext` injection defenses inherited from V6/Phase 4D (keyword patterns,
base64/zero-width detection, sanitization that preserves useful facts).

Tested categories (unit tests, deterministic): forged system/developer instructions,
tool-output impersonation, verified-but-instruction-bearing facts, untrusted sources.
Result: **0 attack successes on the tested set**; benign content preserved.

| Test | Result |
|---|---|
| `TestSearchQuestionDropsInstructionBearingFactEvenWhenVerified` | PASS |
| `TestSearchQuestionTrustAndScopeOnFastPath` | PASS |
| `TestContract_InjectionNoAuthority` (Phase 4D) | PASS |
| `TestSelectContext_InjectionDefense` / `MixedRetainsFact` | PASS |

**NOT TESTED:** encoded/obfuscated attacks beyond base64/zero-width; multi-hop malicious
content; role-delimiter spoofing in the live agent loop.

## E2. Authenticated provenance — NOT IMPLEMENTED

**Current state:** provenance is a plain string tag (`source:verified`,
`source:untrusted`, `source:injection`). It is **not cryptographically authenticated**.
A string such as `source:verified` is not proof and can be forged by anything that can
write the store.

**Proposed design (not implemented):**
- Ingestion identities: each writer holds an Ed25519 key; the public key is registered
  as a trusted source.
- Each memory carries a detached signature over (canonical body + project + timestamp +
  source id); `source:verified` becomes a *verified signature by a trusted key*, not a
  literal string.
- Authorization to write provenance: only holders of a registered key may sign.
- Integrity verification at retrieval: signatures checked before a memory is treated as
  trusted; unsigned/legacy records are downgraded to `unverified`.
- Key management/rotation: keys have validity windows; rotation re-signs on next write.
- Replay protection: include a monotonic counter or nonce per source.
- Legacy handling: unsigned records remain readable but never `verified`.

This is a substantial change and is out of scope for the current hardening pass.

## E3. Isolation (existing coverage)

| Check | Test | Result |
|---|---|---|
| Cross-project leakage | `TestPhase2CrossScopeLeakageAllPaths`, `TestSearchQuestionTrustAndScopeOnFastPath` | PASS |
| Cache isolation | `TestSearchCacheInvalidatedByWrite/Supersede` | PASS |
| Concurrent multi-project access | `TestPhase2ConcurrencyStress` (scopes isolated) | PASS (0 stale) |
| Vector-index isolation | `TestPhase2EmbeddingModelVersionIsolation` | PASS |
| Unauthorized update/delete | not exercised (no auth layer) | **NOT TESTED** |

## Verdict

Injection resistance (tested set): **PASS**. Authenticated provenance: **NOT
IMPLEMENTED** (design proposed). Isolation (project/cache/vector): **PASS**. Cross-tenant
authorization: **NOT TESTED**.
