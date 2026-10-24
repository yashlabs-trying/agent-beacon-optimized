# Beacon V7 hardening — Phase G: ANN and Neural Retrieval

Status: **NOT TESTED in Phase G.** Optional ANN and neural retrieval remain disabled by
default and were not re-benchmarked in this pass.

## 1. Current configuration

- Default V7 path: lexical (`ListMemories`) with question-aware selection
  (`SearchQuestion`). No embedder is configured unless the caller uses
  `OpenSemantic` + `--semantic`.
- Conditional hashed-semantic expansion activates only when trusted lexical evidence
  does not directly support the question.
- ANN (`UseANN`) and real neural (BGE-small via the Mac sidecar) remain opt-in and are
  not part of the default path.

## 2. Prior measurements (carried, not re-run here)

| Mode | Recall@5 (locked 48-query fixture) | Latency | Source |
|---|---:|---|---|
| lexical | 0.167 | ~0.5 ms | Phase 4A |
| hashed-semantic | 0.875 | ~1 ms | Phase 4A |
| hybrid RRF | 0.875 | ~1 ms | Phase 4A |
| real neural BGE-small | 0.979 | ~43 ms (LAN) | Phase 4A |
| ANN 100K M=64 | recall@10 0.9953 | ~1.2 ms search; 102 s build | Phase 3A |

Phase 4B/4C showed neural ≈ hashed on agent tasks, so the added cost was not justified
on that evidence.

## 3. Why not re-run

The Mac sidecar + BGE model were available, but Phase G is explicitly separate from the
default path and the mission prioritises Phases B–F, H–J. Re-running ANN/neural would
not change the default-path decision and is deferred.

## 4. Decision

Keep ANN and neural **disabled by default**. Re-evaluate only if a future agent-task
study shows a measurable benefit over conditional hashed-semantic.

## Verdict

ANN recall/latency, neural quality, index build/size/RSS, and agent-task improvement for
these modes: **NOT TESTED** in Phase G.
