# Roadmap

The work that would move this optimization from "materially improved, not
production-certified" toward a defensible release. Ordered by impact.

## P0 — close release-blocking gaps

1. **Authenticated provenance deployment.** Ship a protected approval authority
   (OS keychain / managed signing key), provision its public key into the
   `TrustRegistry`, and wire CLI/MCP approval paths to sign. Mechanism + adversarial
   tests already pass; only the protected key authority is missing.
   *Evidence:* [`reports/v7-hardening/AUTHENTICATED_PROVENANCE_FIX.md`](../../reports/v7-hardening/AUTHENTICATED_PROVENANCE_FIX.md).
2. **Recommendation-grade agent-task holdout.** A fresh, diverse, model-backed
   evaluation on held-out tasks with cluster-aware confidence intervals and full
   failure analysis — the only way to make any retrieval-quality claim.
   *Currently:* only a 56-task historical pilot with repeated families exists.

## P1 — scale and integration coverage

3. **Scale sweep.** 64/128 clients and 1M/10M memories with pinned seeds and hashes,
   on fixed hardware. *Currently NOT TESTED.*
4. **Full integration.** Complete repository-wide CLI suite, the MCP/OpenCode
   end-to-end test, and cross-platform (macOS/Windows) checks. *Currently PARTIAL.*
5. **Cold first-build elimination.** Maintain the derived trigram index at ingest so
   the first query never rebuilds. *Currently diagnosed, not implemented.*
6. **`SearchQuestion` p95/p99.** Measure percentiles, not just allocations, once a
   quiesced reference host is available.

## P2 — intelligence quality

7. **Semantic ablation.** Fresh, matched ablation of {lexical, question-aware lexical,
   conditional semantic, +sufficiency} on a held-out suite. Only this can justify
   enabling or removing the semantic component. *Currently NOT TESTED.*
8. **Heuristic robustness.** Validate question/bridge parsing across diverse wording;
   reduce false abstentions and contamination.

## P3 — operational maturity

9. **Append-only security audit log** (scoped in the P6 work; not shipped).
10. **Soak testing** (24–72 h) and multi-process writer soak.
11. **Live-agent injection resistance** testing beyond unit-level.

## Commit map

Optimization/hardening commits after V7-dev (`7b605aa84`), oldest first:

```text
e1a2ada6 docs: audit V7 cold retrieval and profile hardening path
e2408766 phaseB: resolve requested attribute from first content noun
2a62fd4f phaseC: cut warm question-aware allocations
11ca834d phaseD-J: concurrency, security, lifecycle, ANN, testing, ablations, gates
d32e9d3b phase1: serialize in-process writes to fix 16-client tail
189c3e9f phase2: pin cold-start penalty to one-time trigram index build
0cdbeb6a phase3-7: cache review, hot-path, semantic ablation, security, stabilization
2bcc2560 phase1: safe local database placement
90b21107 phase2: cut question-aware allocations by 56%
500e2a0e phase3: authenticated memory provenance (Ed25519, content-bound)
7c17c5fd phase4: final three-problem report + combined revalidation
2f148293 security(phase1-4): project authorization + atomic memory lifecycle
e76a9597 security(phase6): bounded search + request-shape limits
37419b62 security(phase6+reports): honor BEACON_MEMORY_DIR in MCP; full report set
ff2b1447 P0: repair supersession regression (RG-1)
df968893 P0 (follow-up): close approval/supersession race
0b6dbe05 P4: cross-process write reliability, bounded retries, index-cache correctness
227ce043 P3: store-level lifecycle authorization
ff2a0a5c P5: protected Ed25519 approval signing
bedd73b1 P6: append-only security audit log
545cf3b9 P7: context-aware deadlines, cancellation, bounded execution
7cbef131 P1: high-performance transactional batch writes
1f3122d9 Batch4-A: native Windows DPAPI protection for the Ed25519 signing key
004f780d Batch4-B: cross-process crash safety and 16/32-writer validation
92e910bf perf(search): batch the trigram selectivity check for long multi-term queries
acf2aaaf feat(mcp): print the effective project scope at startup
db341fbc test(learning): pin the S4 lifecycle trust boundary
```
