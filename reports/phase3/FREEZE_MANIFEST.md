# Beacon V6 Phase 3 freeze — 22 October 2026

## Verified integration snapshot

The exact verified integration **HEAD including the integration report and all
raw evidence** is `209a987bc96c29b5e5d811a794d0357f20214463`. Its parent is
the two-parent code merge `394e631e7ddd98347795253b546ced527b76cf8e`.
The local tag `freeze/beacon-v6-integration-verified-209a987b` points to the
verified report commit. The independent recovery bundle is
`C:\\Users\\<user>\\work\rescue\beacon-v6-integration-verified-209a987b.bundle`
(69,640,496 bytes, SHA-256
`98C63CC6176449F41800CEEB4E6A6F0403397C57DD0A2B6142BA045470670A7A`).
`git bundle verify` confirmed that it contains a complete history. Phase 3
preparation commits descend from this frozen snapshot; the integration raw
evidence and the Phase 1/Phase 2 branch tips remain unchanged.

## Independent original-Beacon baseline

Original Beacon is `99c709081ad49da1c5ade205929b61b19360ed9b`, tree
`1a846f27c7679d644d128d96c3332ba8220645d2`. A separate local clone with
its own `.git` object store is detached at that commit and has no remote:
`C:\\Users\\<user>\\work\rescue\beacon-original-baseline-99c70908`.
It is clean and has a local `frozen/original-beacon-99c70908` tag. Its complete
recovery bundle is
`C:\\Users\\<user>\\work\rescue\beacon-original-baseline-99c70908.bundle`
(69,362,876 bytes, SHA-256
`8B5BDF743A02B17367B2DD7184C3C7E145D137C5D58E625B21105C5726033A8D`).
`git bundle verify` confirmed a complete history. The existing Mac comparison
checkout at `/Users/<user>/beacon-phase1-baseline` was
also verified clean at the same SHA. Benchmark or agent runs should use a
throwaway copy or generated files outside the baseline checkout.

## Held-out fixtures

New, static fixtures are under `reports/phase3/heldout/`, with canonical-LF
SHA-256 hashes in `LOCK_MANIFEST.json`. Run
`python reports/phase3/heldout/validate_lock.py` before and after any Phase 3
evaluation. A changed hash requires a new dataset ID and a new preregistered
evaluation run; do not silently edit labels after seeing outcomes.

| Dataset | Locked size | Intended use |
|---|---:|---|
| `retrieval_context_v1.json` | 12 topics, 48 queries, 48 expanded memories | Real-neural ranking, lexical/hashed comparisons, distinct-finding context retention. |
| `contradiction_v1.json` | 24 labeled pairs, balanced classes | Contradiction recall, precision, project/version boundaries, cross-kind cases. |
| `agent_tasks_v1.json` | 6 objective tasks | Three-arm local agent pilot with hidden evaluator tests. |

These fixtures were locked before any model or merged-code evaluation against
them. Their size supports a pilot and regression gate, **not** a broad quality
claim. Phase 1's 17/40 neural cases, Phase 2's 96 generated ranking queries and
65 contradiction pairs, and the three-task agent smoke are already observed;
use them for development and regression checks, not as unseen final hold-outs.
Before any larger agent or release-quality experiment, curate and lock an
additional task/query set under a new ID, then reserve it for one final
evaluation pass.

## Operational boundary

The current CLI/MCP/dashboard default is lexical and offline. Neural retrieval
and ANN remain opt-in. This freeze changes reports and evaluation fixtures
only; it does not enable either path or alter runtime defaults. The only
execution hosts for Phase 3 are the existing Apple M5 Mac and WSL2 laptop.
No paid compute, remote push, or PR is authorized by this preparation.
