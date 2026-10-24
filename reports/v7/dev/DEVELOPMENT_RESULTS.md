# V7 development results (historical Phase 5 diagnostics)

These data diagnose the candidate. Phase 5 tasks informed the V7 design; none
of these numbers establishes generalization to unseen agent work.

## What changed

The opt-in `SearchQuestion` path takes a terse retrieval query **and** the full
question. It searches the original-compatible indexed lexical path first,
enforces trust, keeps direct support, and calls semantic search only when direct
support is absent. A semantic candidate needs direct entity/fact support or an
exact named bridge to lexical evidence. No RRF weight tuning, extra model
router, or new dependency was added. The original `ListMemories` path is
unchanged. Context budgeting retains whole memories.

The `--question-aware` CLI flag exposes this path; `--semantic` explicitly
permits local hashed expansion. Embedding model and route are recorded. Missing
project embeddings are backfilled on demand up to 1,000 memories, after which
the caller gets an explicit rebuild error. A lexical writer invalidates an
existing derived vector for updated content.

## Historical selection diagnostic (300 tasks)

Source: Phase 5 fixture and V7 raw `raw/phase5_historical_selection.json`,
SHA-256 `eb4fcbc6dc923e5654a4f02879f09d5bcfbf8cf314c3783a351956c398e9b286`.
This uses `hashed-subword-v1`, not the neural sidecar. Exact means the selected
ID set equals the fixture's required-evidence ID set; that is a diagnostic
measure, not an agent-success score.

| Historical family | Exact selected evidence | Route |
|---|---:|---|
| A3 knowledge transfer | 20/20 | lexical |
| B2 ambiguity | 20/20; ten unresolved questions returned no note | lexical or semantic |
| B3 multi-hop | 20/20 | semantic bridge |
| B5 adversarial trust | 20/20 | lexical + trust |
| C3 destructive-action guard | 20/20 | lexical + trust |

The V7 selector also retained every required evidence ID in A2, A4, A5, B1,
B4, C1, C2, C4 and C5. It passed extra failed-attempt evidence in A1 and old
pre-crash evidence in A4; those are remaining context-precision defects.

## Paired local-model pilot (80 already-seen tasks)

Raw results: `raw/historical_agent_pilot.json`, SHA-256
`1553a9f7f9448142ab8f6c1a56ef7ac9fb95b255636a5b9405aac6d654cfec4e`.
The pilot reused Phase 5's exact system prompt, grader and local
`qwen2.5-coder:7b` model, with randomized arm order. Each task saw the same
memory history; only selected notes differed. The original arm used the
previously recorded original-Beacon selection IDs, not a fresh CLI query.

| Family (20 tasks each) | Original correct | V7 correct |
|---|---:|---:|
| A3 knowledge transfer | 20 | 20 |
| B2 ambiguity | 20 | 20 |
| B3 multi-hop | 0 | 7 |
| B5 adversarial trust | 14 | 20 |
| **Total** | **54/80** | **67/80** |

Paired discordance: V7 wins 13, loses 0, exact McNemar p=0.000244 **at the
task-row level**. Many rows repeat the same mechanism; that p-value is not
evidence of 80 independent real workflows. Model-only mean latency was 0.497 s
for V7 versus 0.476 s for original; mean prompt tokens were 123.4 versus
121.9. Retrieval time was not measured in this pilot, so end-to-end speed and
cost improvements are **unproven**. The later uncertainty-qualification change
left all 300 historical selection ID lists byte-identical to the pilot input.

## Validation status

- Focused V7 search, trust, scope, budget, semantic backfill and stale-vector
  tests passed on WSL2. The full learning package passed before the last
  uncertainty-qualification refinement; its focused and race tests passed
  afterward.
- Go vet passed for `internal/learning` and `cmd`. V7 CLI built locally.
- Full CLI tests initially failed with the repository-standard placeholder
  `hooks.bin`. After building the real local hooks binary, the remaining
  `TestMCPConnectEndToEndOpenCode` failed with “No MCP servers configured”.
  The same test failed identically at the untouched Phase 5 checkout (raw
  `phase5_opencode_control.log`); it is not attributable to this V7 path.
- A new 56-task pilot holdout was generated and locked separately before any
  model run. Its result belongs in `../validation/`, not in these development
  metrics.
