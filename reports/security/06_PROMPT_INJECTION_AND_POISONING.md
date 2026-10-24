# 06 — Prompt Injection and Poisoning Containment (Security Layer 4)

Status: **existing defenses retained and re-verified**; **containment improved**
at the authorization boundary; **model-backed agent-behavior evaluation NOT
TESTED** (no model available in this environment).

## Distinction kept

Retrieved memory is **data**, not a trusted control instruction. Original Beacon
already ships injection detection; V7 keeps it and adds containment at the
retrieval boundary rather than only adding more patterns.

## Existing defenses (verified present, not credited as new)

| Defense | Location |
|---|---|
| Instruction-bearing evidence dropped before it becomes an answer | `search_question.go` (`TestSearchQuestionDropsInstructionBearingFactEvenWhenVerified`) |
| Injection pattern + base64/zero-width detection, sanitization | `context_select.go` (`SelectTrustContext`, `TestSelectContext_InjectionDefense`, `TestContract_InjectionNoAuthority`) |
| Project/scope isolation on retrieval | `search_question.go`, `context_select.go`, `TestSearchQuestionTrustAndScopeOnFastPath` |
| Hostile synthetic payload round-trip and no cross-project leak | `security_smoke_test.go` (`TestAdversarialMemoryRetrieval`) |

`security_smoke_test.go` exercises inert synthetic hostile bodies (fake
`[SYSTEM]` text, tool-manipulation text, synthetic exfiltration target, forged
`approved=true`, SQL payload, 256 KB body, malformed metadata) and asserts
(a) faithful round-trip, (b) no cross-project leakage in search, (c) the SQL-like
payload does not alter the authoritative table.

## Containment added this pass

- **Authorization before retrieval** (report 03): poisoned content in a project
  the caller cannot access can no longer be retrieved by ID or by project
  substitution. This is containment, not detection: even a perfectly
  innocuous-looking malicious memory cannot cross a project boundary.
- **Bounded input** (report 07): oversized query/term/id inputs are rejected, so
  a single call cannot be used to pull a huge context window.
- **No external tool execution on the retrieval path**: retrieval performs no
  tool calls, network calls, or command execution.

## What is NOT established

- **Attack-success rate against a live agent.** A lexical detector passing its
  unit tests is not proof the agent resists injection. No model-backed
  controlled evaluation was run (no model is available locally, and paid compute
  is out of scope). This is reported as **NOT TESTED**, not as a pass.
- **False-positive / false-negative rates on a large corpus.** Not measured here.
- **Encoded/obfuscated payloads beyond base64/zero-width**, multi-hop malicious
  content, and role-delimiter spoofing in the live loop remain **NOT TESTED**
  (consistent with the prior V7 report E1).
- **Skill promotion.** Promoting retrieved content to an executable Agent Skill
  still requires an approved candidate (`skills.go`,
  `TestCandidateMemoryForSkillRequiresApprovedCandidate`); human approval for
  dangerous promotions is unchanged. A model-backed skill-hijack evaluation is
  NOT TESTED.

## Acceptance

| Requirement | Status |
|---|---|
| Retrieved memory is data, not instruction | PASS (existing, re-verified) |
| No cross-project poisoned retrieval | PASS (new authorization) |
| No tool execution on retrieval path | PASS (by construction) |
| Human approval for skill promotion | PASS (existing) |
| Improved containment without losing legitimate retrieval | PARTIAL — containment improved; legitimate-retrieval cost measured only as the O(1) authorization lookup |
| Model-backed injection resistance | **NOT TESTED** |
