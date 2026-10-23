# Phase 5 — Coding and Workflow (Suite C)

## Suite C, per category

| Category | A | B | C | D | E |
|---|---:|---:|---:|---:|---:|
| C1 coding (hidden tests) | 0/20 | 13/20 | 13/20 | 13/20 | 13/20 |
| C2 workflow | 0/20 | 20/20 | 20/20 | 20/20 | 20/20 |
| C3 security/ops restraint | 20/20 | 20/20 | 20/20 | 20/20 | 20/20 |
| C4 long-history selection | 0/20 | 20/20 | 20/20 | 20/20 | 20/20 |
| C5 cross-project | 0/20 | 19/20 | 19/20 | 19/20 | 19/20 |
| **Suite C total** | 20/100 | 92/100 | 92/100 | 92/100 | 92/100 |

## Findings

1. **C1 coding**: graded by executing hidden unit tests (not by explanation). All memory
   arms fix **13/20**; no-memory fixes 0/20 (the repaired prompt still requires memory for
   the convention, but the code itself is small). The 7 failures are model coding errors
   (e.g., clamp bounds), **not memory failures** — the same code is retrieved for every
   memory arm.
2. **C2/C4/C5 are saturated for memory arms** (20/20): workflow ordering, long-history
   selection, and cross-project isolation are all solved once any retrieval works. C5
   (19/20) and C3 (20/20) do not discriminate.
3. **C3 security restraint: all arms 20/20**, including no-memory — the destructive-action
   guard note makes every configuration refuse. This category does not test the memory
   system's security; the discriminating security test is **B5** (see `SECURITY_AND_TRUST.md`).

## Answer

- Does V6 fix real coding tasks better? **NO** — identical to original (13/20 each).
- Does V6 handle multi-step workflows / cross-project isolation better? **EQUIVALENT**.
- The Suite C ceiling (92/100 for all memory arms, oracle also 92) shows Suite C is
  largely saturated and contributes little discrimination.

## Note on validity

C1's `none` arm scores 0/20 because the convention is memory-provided; the coding work
itself is small. C1 is a valid memory-dependent coding task but is not a discriminator
between memory systems (all equal).
