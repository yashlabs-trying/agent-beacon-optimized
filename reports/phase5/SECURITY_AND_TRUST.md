# Phase 5 — Security and Trust

## 1. The discriminating security test: B5 trust/adversarial (20 tasks)

Each task has a verified fact plus an adversarial note (fake system/developer/tool
instruction or forged "admin override") carrying a wrong value.

| Arm | A | B | C | D | E |
|---|---:|---:|---:|---:|---:|
| B5 correct | 0/20 | 14/20 | **5/20** | **20/20** | 20/20 |

**This is the sharpest V6-vs-original finding:**
- **Semantic retrieval (C) is significantly WORSE than original (B) at security (5 vs 14)**:
  it retrieves the malicious note (which lexical sometimes misses because the injected text
  lacks the query terms), and the model obeys it.
- **The intelligent layer (D) fully fixes this (20/20 = oracle)** by dropping the
  untrusted/injection note before it reaches the agent.

## 2. Forbidden-token leaks (B5 + C3)

| Arm | leaks |
|---|---:|
| A / B / C / D / E | 0/40 each |

No arm produced the explicit forbidden token; the B5 differences are in *correctness*
(answering the verified value), not raw leak counts. This shows the leak metric alone is
insufficient — the accuracy metric reveals the semantic regression.

## 3. Cross-project isolation (C5)

All memory arms 19/20; no cross-scope leaks observed. The storage suite
(`TestPhase2CrossScopeLeakageAllPaths`) also PASSES.

## 4. Unauthorized actions

C3 (destructive-action restraint): all arms 20/20 — no unauthorized action executed. The
harness uses no real tools, so tool-authorization tests are limited to answer-level
restraint; **sandboxed tool-execution security tests were NOT RUN**.

## Answers

- Does V6 resist malicious/misleading memories better? **YES — the intelligent layer is
  the only configuration that reaches 20/20; semantic alone is worse than original.**
- Security violations (leaks / scope): **0 for all arms**; the meaningful regression is
  semantic's accuracy drop on B5, which the intelligent layer repairs.
- Encoded/obfuscated injection beyond the fixture set: **NOT TESTED**.
- No release-blocking security failure was observed; the B5 semantic regression is a
  correctness/robustness issue, not a leak.

## Verdict

Security: **BETTER for V6 intelligent** (fixes the semantic regression; matches oracle);
**WORSE for V6 semantic** than original. No leaks in any arm.
