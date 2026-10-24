# Phase 5 — Release Gates

Pre-registered targets are ambitious engineering acceptance targets, not assumptions.
Status is reported against measured denominators.

| Gate | Target | Measured | Status |
|---|---|---|---|
| Agent intelligence: V6 ≥ original +10 pts on hard memory-dependent tasks | +10 pts | **−1.7 pts (224 vs 229)** | **FAIL** |
| Contradiction P/R | ≥95% / ≥95% | locked P=1.000, R=0.500 default (cross-kind opt-in 1.000); agent B1 8/20 all arms | **FAIL** |
| Ambiguity correct behavior | ≥95% | B2: original 20/20, semantic/intelligent 16/20 | **FAIL** |
| Evidence grounding | ≥95% supported | required-evidence recall 1.000 (C/D/E); answer support limited by reasoning | **PARTIAL** |
| Reliability: no acknowledged-write loss | 0 loss | crash/durability suites PASS | **PASS** |
| No silent project-boundary leakage | 0 | C5 19/20, `TestPhase2CrossScopeLeakageAllPaths` PASS | **PASS** |
| No unauthorized security-sensitive tool action | 0 | C3 20/20 all arms; leaks 0/40 all arms | **PASS** |
| Recovery for supported faults | auditable | crash/index/restart suites PASS | **PASS** |
| Speed: agent-level latency improvement | measurable | D 0.92 s vs B 1.18 s (−22%) | **PASS** |
| Efficiency: fewer tokens/wasted calls | fewer | D tokens 149 vs B 131 | **FAIL (tokens worse)**; time/success better |
| V6 intelligent vs V6 semantic | better | 15–0–285, p=0.0001 | **PASS** |
| V6 intelligent vs original | better | 17–22–261, p=0.52 | **FAIL (tied)** |

## Security/integrity as release-blocking

No security or data-integrity violation was observed (0 leaks, 0 acknowledged-write loss,
0 scope leaks). Therefore no release-blocking *safety* failure; however, the **primary
intelligence gate FAILS** (no improvement over original), and the **contradiction and
ambiguity gates FAIL**.

## Bottom line

The system **cannot be declared superior to original Beacon** on this evaluation. The
intelligent layer is a genuine improvement over V6 semantic and over the security
regression semantic introduced, but it does not exceed the original lexical baseline
overall, and it regresses on precision/ambiguity categories.
