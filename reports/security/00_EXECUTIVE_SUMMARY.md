# Executive Summary — Beacon V7 Security Hardening

Baseline: `7c17c5fd1503769e0fc147c0f9d3c551ee89c539` (verified HEAD at start).
Frozen original: `99c709081ad49da1c5ade205929b61b19360ed9b`.
Hardening branch: `codex/beacon-v7-security-hardening` (local commits only; no
push, no PR, no remote testing).

## What was done this pass

- Phase 0: verified git state, froze baseline tags, checked cross-commit source
  identity, made the test build work on Windows.
- Phase 1: reproduced Finding A (unscoped `get_memory`, caller-controlled project
  scope) and Finding C (O(N) text scan) with tests and a benchmark.
- Phase 2: server-side project authorization with a fail-closed allowlist,
  project-scoped `get_memory`/search/context, and a shared (non-leaking) store.
- Phase 4: atomic, race-safe candidate approval/rejection/supersession with
  crash-injection proof.
- Phase 6: bounded search (explicit overload error) and request-shape limits.
- Fixed a filesystem-consistency bug: the MCP server now honors
  `BEACON_MEMORY_DIR`.
- Verified the existing Ed25519 provenance mechanism and injection defenses.

## The 12 questions

1. **Which weaknesses were confirmed in original Beacon?** Finding A
   (cross-project access via unscoped `get_memory` and caller-supplied project
   scope) and Finding B (non-atomic approval/supersession) are confirmed;
   Finding C (O(N) text scan) is confirmed as a performance risk. A and B are
   byte-identical between the frozen original and the V7 baseline.

2. **Which weaknesses were confirmed in V7?** The same three: V7 added no
   authorization and no atomicity; its FTS5 search mitigates but does not remove
   the unbounded scan for non-selective queries.

3. **Which exact problems did we fix?** Cross-project memory access (with an
   allowlist); atomic candidate/memory transitions; unbounded non-selective
   scans; oversized query/term/id inputs; a per-call `*sql.DB` pool leak; the
   MCP server ignoring `BEACON_MEMORY_DIR`.

4. **Which attacks now fail against V7?** Cross-project ID lookup and project
   substitution (allowlist mode); concurrent double-approval; approving a
   rejected candidate; crash-induced half-applied review state; forged/tampered
   provenance; oversized input; pathological broad scans.

5. **Which attacks still succeed?** In the documented single-user local default,
   cross-project access is not enforced (by design). Without a protected signing
   authority, an attacker who can write the store can write arbitrary
   unattested content (but cannot gain Verified status). Live-agent prompt
   injection resistance is untested.

6. **Which defenses already existed in original Beacon?** Injection detection
   and the "retrieved memory has no authority" contract; project scoping for
   search (not for ID lookup); `ProjectIDForPath` path-safety; the Ed25519
   provenance primitive (added in V7 phase 3); cache generation invalidation;
   `BEACON_MEMORY_DIR` placement control.

7. **What additional security overhead did we introduce?** One O(1) map lookup
   per authorization decision and per returned row; a bounded scan counter; input
   validation. Warm retrieval allocations measured at 898/op vs a reported ~875
   (within noise). Approval now uses one transaction instead of two.

8. **Is V7 still faster?** On the retrieval hot path, yes/unchanged: indexed
   no-match search is ~flat in N (81→142 µs, 82 allocs) versus the original scan
   (8.05→46 ms, 12k→61k allocs). The 16-client write p99 gate could not be
   verified on the reference (Linux) platform from this Windows environment.

9. **Can V7 safely serve multiple agents?** Yes **if** the operator sets a
   project allowlist (or each agent is a separate OS user/DB). Not automatically:
   the default mode is single-user local. Authenticated shared-HTTP caller
   identity is BLOCKED.

10. **Is the database still reliable under failure?** Yes for the tested crash
    matrix (atomic lifecycle, WAL recovery, index rebuild). Multi-process writer
    correctness is NOT TESTED.

11. **Is V7 demonstrably better on the tested security scenarios?** Yes — on the
    authorization, atomicity, resource-bound, and provenance scenarios with
    reproducible tests. Model-backed injection resistance and large-scale
    scalability are not established.

12. **Which production gates remain unfinished?** Protected approval authority
    (BLOCKED); security audit log (NOT IMPLEMENTED); multi-process correctness
    (NOT TESTED); reference-platform performance gate (NOT VERIFIED); 1M/64-client
    scalability (NOT TESTED); full CLI/cross-platform integration (PARTIAL).

## Side-by-side

| Dimension | Original | V7 baseline | Hardened branch |
|---|---|---|---|
| Cross-project ID lookup | exposed | exposed | enforced (allowlist) |
| Caller-controlled project scope | trusted | trusted | authorized |
| Approval atomicity | no | no | yes (one tx) |
| Concurrent double-approval | unguarded | unguarded | single winner |
| Provenance | string tag | Ed25519 (opt-in) | Ed25519 (verified) |
| Approval signing authority | none | BLOCKED | BLOCKED |
| Broad/no-match search | O(N) scan | FTS5 + scan | FTS5 + bounded scan |
| Oversized input | accepted | accepted | rejected |
| Per-call DB pool | leaked | leaked | shared + Close |
| MCP placement override | ignored | ignored | honored |
| Security audit log | none | none | none (NOT IMPL) |
| Injection (unit) | pass | pass | pass (retained) |
| Live-agent injection | not tested | not tested | not tested |

## Verdict

The hardening branch fixes the two confirmed original authorization/reliability
defects and the confirmed performance risk, with reproducible tests, while
preserving the existing retrieval intelligence and adding no new services. It is
**not** production-certified: authenticated provenance's approval authority, the
audit log, multi-process correctness, and reference-platform performance remain
open gates. No claim of invulnerability, infinite scalability, or certification
is made.

If the confirmed original findings (A and B) are deemed disclosable, the private
reproduction evidence in this directory is sufficient for a coordinated report
through the maintainers' security channel; **no contact was made**.
