# V7 independent pilot lock

Locked before any model call against this fixture.

- Fixture: `holdout_v1.json`
- SHA-256 of exact UTF-8/LF bytes:
  `0c55552721e1ae0dedb979bd9eabdced6e03eeb440ca38e7bb7209d7013ad044`
- Generator: `generate.py`, deterministic and dependency-free.
- Size: 56 tasks, seven mechanism families with eight distinct projects each.
- Original Beacon source: `99c709081ad49da1c5ade205929b61b19360ed9b`.
- V7 candidate starts from Phase 5 `02e34f98dec1c5d34a2f0d52904969edc39e389f`.
- Local model: `qwen2.5-coder:7b`, temperature 0, seed 0, context 4096,
  prediction cap 384. Both arms use the same Phase 5 system prompt and grader.
- Retrieval: actual original `beacon memory list` binary and the V7 opt-in
  `--question-aware --semantic` CLI, each run against an isolated project store
  seeded from the same memory history. The agent never sees gold tokens,
  forbidden tokens or required-evidence IDs.
- Execute paired tasks in randomized arm order; record selection IDs, answers,
  grading, prompt tokens and end-to-end latency. Preserve every raw result.

This is a **pilot holdout**, not a statistically independent 56-workflow sample:
each family shares a task mechanism. Report both task-level and family-level
outcomes, including every loss. Do not tune this candidate on the results and
then call a rerun on this fixture independent. A final superiority claim needs
a larger, more diverse untouched set of real agent workflows.
