# Phase 5 — Statistical Analysis

## 1. Unit and aggregation

- Unit = logical task (300). Seeds are correlated repetitions of the same task; seed 0 is
  reported, with seeds 1–2 verified identical on a 100-task subset.
- Task-level correctness = any-seed correct (identical to per-seed here).

## 2. Per-arm accuracy with Wilson 95% CI

| Arm | Correct | Acc | 95% CI |
|---|---:|---:|---|
| A none | 30/300 | 0.100 | [0.071, 0.139] |
| B original | 229/300 | 0.763 | [0.712, 0.808] |
| C semantic | 209/300 | 0.697 | [0.642, 0.746] |
| D intelligent | 224/300 | 0.747 | [0.695, 0.793] |
| E oracle | 246/300 | 0.820 | [0.773, 0.859] |

The B and D CIs overlap heavily → no established difference.

## 3. Paired comparisons (exact McNemar, task-level)

| Comparison | A / B / ties | p | Conclusion |
|---|---|---:|---|
| D vs B | 17 / 22 / 261 | 0.5224 | **no difference** |
| D vs C | 15 / 0 / 285 | 0.0001 | D better |
| C vs B | 11 / 31 / 258 | 0.0029 | **C worse** |
| D vs A | 198 / 4 / 98 | <0.0001 | memory helps |
| B vs A | 199 / 0 / 101 | <0.0001 | memory helps |
| D vs E | 6 / 28 / 266 | 0.0002 | oracle better |

## 4. Per-suite paired (D vs B)

| Suite | D wins | B wins | ties | p |
|---|---:|---:|---:|---:|
| A | 3 | 18 | 79 | 0.0015 (B better) |
| B | 14 | 4 | 82 | 0.0309 (D better) |
| C | 0 | 0 | 100 | 1.000 |

**The tie overall masks opposing suite effects:** B wins Suite A (driven by A3), D wins
Suite B (driven by B5). This is the key statistical structure of the result.

## 5. Seed variability

Seeds 0/1/2 identical (temperature 0). Effective independent trials = 1; conclusions are
"preliminary" with respect to seed sampling, though the effect direction is stable.

## 6. Small-sample caveats

Several categories have n=20 (per arm). Category-level CIs are wide; category conclusions
are directional. Contradiction (8/20) and multi-hop (7–8/20) are at the model ceiling.

## 7. What is and is not established

- **Established:** memory >> no memory; D > C; C < B; D ≈ B overall.
- **Not established:** any D > B advantage; any V6 superiority; seed-level robustness
  beyond determinism.
