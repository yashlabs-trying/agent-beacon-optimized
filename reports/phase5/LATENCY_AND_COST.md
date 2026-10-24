# Phase 5 — Latency and Cost

## 1. Per-run agent latency and tokens (300 tasks × 5 arms)

| Arm | mean agent s | mean prompt tokens | mean gen tokens |
|---|---:|---:|---:|
| A none | 0.89 | 113 | ~8 |
| B original | 1.18 | 131 | ~10 |
| C semantic | 1.13 | 151 | ~10 |
| **D intelligent** | **0.92** | 149 | ~10 |
| E oracle | 0.90 | 126 | ~10 |

- **D is ~22% faster than B per task** (0.92 s vs 1.18 s) — the intelligent layer returns
  fewer, curated memories, shortening generation.
- **D uses ~14% more prompt tokens than B** (149 vs 131) because semantic retrieval
  surfaces more candidates before selection. C uses the most (151).
- Oracle is fastest/lowest-token (0.90 s, 126) as expected.

## 2. Selector and retrieval overhead

- Context selection (indexed): sub-6 ms at n=1000; negligible at the K=8 used here.
- Retrieval: hashed ~1 ms; neural sidecar ~43 ms/query over the LAN (not used by arm D in
  this campaign — D used offline hashed). Neural is **NOT TESTED** in the agent campaign.

## 3. Cost per successful task (local model, no monetary cost)

| Arm | success | mean s/task | s per correct task |
|---|---:|---:|---:|
| B original | 229/300 | 1.18 | 1.55 |
| C semantic | 209/300 | 1.13 | 1.62 |
| D intelligent | 224/300 | 0.92 | **1.23** |
| E oracle | 246/300 | 0.90 | 1.10 |

**D has the lowest time-per-success of the memory systems** (1.23 s vs B 1.55, C 1.62),
because it is faster and nearly as accurate as B. In token terms D is slightly worse per
task but better per success than C.

## 4. Honest framing

- The Phase 4D "31.5×/36.2× selector speedup" is a **microbenchmark**; end-to-end it is
  invisible at K=8. Agent-level latency is dominated by model generation, not selection.
- No monetary cost applies (local model); cost is reported as time/tokens only.

## Verdict

End-to-end latency: **BETTER for D** (0.92 s vs B 1.18 s). Tokens: **WORSE for D** than B
(149 vs 131), better than C per success. Microbenchmark speedup does **not** translate to
agent intelligence.
