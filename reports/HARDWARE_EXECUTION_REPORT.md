# Beacon V6 — Hardware Execution Report

Date: 2026-10-21. Branch: `feat/beacon-memory-engine-v6-production` (from `6bf57b9a`).

## 1. Experiments that ran locally (WSL2 Ubuntu, i5, 8 vCPU, 8 GB RAM)

All V2–V5 work is local and already committed; it is the reference evidence for
this report:

- Full learning test suite, `-race` (647 s), build, vet, gofmt.
- Lexical scaling 1K–1M (1M DB 1.35 GB), semantic exact 1K–100K.
- ANN recall fix: recall@10 0.17 → 0.975 (M=64), 72× faster than exact at 100K.
- Trust, context budgeting, concurrency, cache/fallback tests.

## 2. Experiments that ran on the RTX 3090

**NONE.** Remote access could not be established, so no pod experiment ran and no
pod result is claimed.

## 3. GPU utilization and runtime

- GPU utilization: **0%** — the pod was never reachable.
- Pod rental cost attributable to this work: **$0** (no workload executed).

## 4. Results and comparisons

No cross-machine comparison is reported, because none was performed. The local
V5 numbers stand as previously reported (`V5_CONSOLIDATED_BENCHMARKS.md`).

## 5. Estimated compute expenditure

- Local: existing machine, no marginal cost.
- GPU: **$0** (blocked).

## 6. Tests that could not complete

Everything assigned to the GPU server: neural embedding inference, semantic
benchmarks, 100K–1M ANN, 1M lexical under 125 GB RAM, GPU coding-agent
evaluations, high-concurrency stress, large-scale durability/crash recovery.

## 7. Is a larger GPU necessary?

No. The offered RTX 3090 (24 GB VRAM, ~125 GB RAM) is **sufficient** for
embeddings (bge-small/MiniLM), 1M ANN (RAM-bound; 125 GB removes the wall that
blocked the 8 GB laptop), and a 7–14B agent pilot. A larger GPU (48 GB) would
only raise the ceiling for stronger agent models; it is not required to execute
the plan.

## 8. Remaining production blockers

1. **SSH access** (immediate blocker — see below).
2. No neural embedding backend integrated.
3. No measured agent-task success.
4. 1M ANN and crash/soak not yet run (RAM/time).

## Precise connection blocker

Both local keys are rejected by the pod, on both endpoints:

```
ssh -i ~/.ssh/id_ed25519      root@213.192.2.119 -p 40197   -> Permission denied (publickey,password)
ssh -i ~/.ssh/runpod_kernellab root@213.192.2.119 -p 40197  -> Permission denied (publickey,password)
ssh -i ~/.ssh/id_ed25519      <pod-id>@ssh.runpod.io -> Permission denied (publickey)
ssh -i ~/.ssh/runpod_kernellab <pod-id>@ssh.runpod.io -> Permission denied (publickey)
```

Local public keys present: `id_ed25519` (`user@host`) and
`runpod_kernellab`. Neither is in the pod's `authorized_keys`. This is an
access-provisioning problem, not a network problem (TCP to `213.192.2.119:40197`
connects; the host key was added).

**Fix:** register one of these public keys for the pod in the RunPod console (or
paste the pod's expected public key here), then re-run `reports/v6_pod_runbook.sh`.

## Prepared remote execution

`reports/v6_pod_runbook.sh` is ready to run on the pod: it checks the
environment, installs Go 1.27 if absent, checks out the branch, runs the test
suite, then the 1M lexical and 1M ANN benchmarks, writing results to
`$HOME/v6-results/`. It performs no destructive operations and uses disposable
databases.
