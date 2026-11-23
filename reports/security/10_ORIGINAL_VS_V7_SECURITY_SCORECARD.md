# 10 — Original Beacon vs V7 Security Scorecard

Legend: **BLOCKED** = attack stopped; **SUCCEEDED** = attack worked;
**INCONCLUSIVE**; **N/A** = feature absent in that build; **NOT TESTED**.

| ID | Threat / scenario | Original result | V7 baseline result | Hardening result | Original mitigation | V7/hardening mitigation | Impact | Confidence | Severity | Perf overhead | Evidence |
|---|---|---|---|---|---|---|---|---|---|---|---|
| AUTH-01 | Unscoped `get_memory` reads another project's memory | SUCCEEDED | SUCCEEDED | **BLOCKED** (allowlist configured) | none (path-safety only) | `authorizeProject` on the memory's own project | Confidentiality | High (repro test) | High in multi-scope; Low local | O(1) map lookup | E-03, E-05 |
| AUTH-02 | Caller-supplied `project_id` substitution in search | SUCCEEDED | SUCCEEDED | **BLOCKED** | none | authorize resolved project + post-filter | Confidentiality | High | High in multi-scope | O(1)+O(rows) | E-03, E-05 |
| AUTH-03 | Caller-supplied `project` path substitution | N/A (path maps to nothing unknown) | SUCCEEDED for known projects | **BLOCKED** | `ProjectIDForPath` (not authz) | authorize resolved ID | Confidentiality | Medium (by inspection) | Medium | O(known projects) | report 03 |
| AUTH-04 | Empty/unknown project scope widens access | n/a | not widened (scopes to nothing) | **BLOCKED** (fail-closed) | unknown path → no rows | empty allowlist denies all | Confidentiality | High | Medium | none | E-05 |
| AUTH-05 | Shared HTTP caller without identity | local only | local only | **PARTIAL** (loopback + optional allowlist; no caller auth) | loopback + Host check | warning + allowlist | Confidentiality | Medium | Medium | none | report 03 |
| LIFE-01 | Crash between memory write and candidate transition | SUCCEEDED (inconsistent) | SUCCEEDED | **BLOCKED** | idempotent IDs only | one transaction + conditional state | Integrity | High (crash matrix) | High | ≤ (one tx) | E-06 |
| LIFE-02 | Concurrent reviewers double-approve | INCONCLUSIVE (idempotent) | INCONCLUSIVE | **BLOCKED** (single winner) | deterministic IDs | conditional update | Integrity | High | Medium | none | E-06 |
| LIFE-03 | Approve a rejected candidate | SUCCEEDED (pre-check) | SUCCEEDED | **BLOCKED** | state pre-check | conditional update | Integrity | High | Medium | none | E-06 |
| PROV-01 | Forged `source:verified` label | SUCCEEDED | BLOCKED (unverified) | **BLOCKED** | none | Ed25519 attestation | Integrity | High | High | ~µs when enabled | E, report 04 |
| PROV-02 | Tampered body after signing | N/A | BLOCKED (invalid) | **BLOCKED** | none | content digest binding | Integrity | High | High | ~µs | report 04 |
| PROV-03 | Cross-project attestation replay | N/A | BLOCKED (invalid) | **BLOCKED** | none | project binding | Integrity | High | Medium | ~µs | report 04 |
| PROV-04 | Revoked key still trusted | N/A | BLOCKED (not verified) | **BLOCKED** | none | registry `Remove` | Integrity | High | High | ~µs | report 04 |
| PROV-05 | End-to-end protected approval authority | N/A | N/A | **BLOCKED** (not shipped) | none | none | Integrity | High | High | — | report 04 |
| INJ-01 | `[SYSTEM]`-style instruction in memory body | BLOCKED (retrieval) | BLOCKED | **BLOCKED** | detection + no-authority contract | retained | Integrity | Medium (unit) | High | none | report 06 |
| INJ-02 | Malicious memory in another project | SUCCEEDED | SUCCEEDED | **BLOCKED** | none | authorization | Confidentiality | High | Medium | O(1) | report 03/06 |
| INJ-03 | Live-agent injection resistance | NOT TESTED | NOT TESTED | **NOT TESTED** | — | — | Integrity | — | — | — | report 06 |
| RES-01 | No-match / broad text search | O(N) scan (46 ms @5k) | bounded (indexed) | **BLOCKED** (row cap) | none | FTS5 + scan cap | Availability | High | Medium | indexed ~flat | E-04, report 07 |
| RES-02 | Oversized query / id | ACCEPTED | ACCEPTED | **BLOCKED** (rejected) | none | input limits | Availability | High | Medium | none | report 07 |
| RES-03 | Per-call pool leak | leaked | leaked | **BLOCKED** (shared store) | none | cached store + Close | Availability | Medium | Medium | improved | report 07 |
| RES-04 | Request deadline / cancellation | N/A | N/A | **NOT TESTED** | none | none | Availability | — | — | — | report 07 |
| CACHE-01 | Cache bypasses authorization | N/A | N/A | **BLOCKED** | generation invalidation | pre+post authorization filter | Confidentiality | Medium | Medium | O(rows) | report 08 |
| CACHE-02 | Cache survives write/supersede | BLOCKED | BLOCKED | **BLOCKED** | generation bump | retained | Integrity | High | Medium | none | report 08 |
| FS-01 | Placement override ignored by MCP | SUCCEEDED (inconsistent) | SUCCEEDED | **BLOCKED** (fixed) | `BEACON_MEMORY_DIR` | `DatabasePath` | Integrity | High | Low | none | report 08 |
| AUD-01 | Security audit record | N/A | N/A | **NOT IMPLEMENTED** | — | — | Integrity | — | — | — | report 09 |

## Notes

- "Original result" for AUTH/LIFE rows is reproduced on the frozen commit by
  source identity (byte-identical `server.go`/`candidate.go`) and the baseline
  repro test.
- A runtime error is not counted as enforcement: AUTH-01/02 are counted BLOCKED
  only because the denial was observed **and** the invariant (no memory content
  in the response) was checked.
- No attack in this table is described as remote; all are local to the declared
  threat model.
- The hardening branch's default mode remains the documented single-user local
  policy, where AUTH-01/02 are not enforced (by design, not claimed).
