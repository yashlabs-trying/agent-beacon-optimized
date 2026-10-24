# Phase 4B — Harness Audit (Stage A)

Before building anything, the Phase 4A harness and reports were inspected to find
what is trustworthy, what is redundant, and — critically — what the existing harness
is *structurally incapable* of measuring.

Audited artifacts:
- `reports/phase3/part4/harness/agent_eval.py` (six-task three-arm agent harness)
- `cli/beacon/internal/learning/phase3a_retrieval_test.go`, `phase3a_semantic_test.go`
- `reports/phase4/part1/harness/retrieval_baselines.py`, `validate_harness.py`
- Phase 4A reports (`SIX_TASK_PILOT.md`, `RETRIEVAL_BASELINE_COMPARISON.md`, …)

## 1. What is already tested

| Capability | Tested? | Where | Trustworthy? |
|---|---|---|---|
| Functional task success (hidden unittest) | Yes | `agent_eval.py` | Yes |
| Harness isolation / determinism / negative control | Yes | `validate_harness.py` (16/16) | Yes |
| Retrieval recall/latency on labeled queries | Yes | `phase3a_*_test.go` | Yes |
| Independent lexical baselines (BM25/FTS5/recency) | Yes | `retrieval_baselines.py` | Yes |
| Contradiction precision/recall (locked 24 pairs) | Yes | `phase3a_contradiction_test.go` | Yes |
| Crash/durability/concurrency | Yes | `phase2/phase3*_test.go` | Yes |
| **Memory-system ranking differences → agent outcome** | **No** | — | **structurally absent** |
| Contradiction / ambiguity / abstention *agent* tasks | No | — | absent |
| Evidence-utilization (did the agent use the retrieved fact?) | No | — | absent |
| Distractor resistance at scale | No | — | absent |

## 2. Structural blind spots (why the six-task pilot told us little)

1. **Two memories per task.** `phase3dseed` inserts exactly `<task>-current` and
   `<task>-distractor`. With `--limit 5`, *both* are always returned regardless of
   ranker. Original and V6 therefore receive **byte-identical evidence** — the harness
   cannot, even in principle, show a retrieval-quality difference. This was observed:
   both arms retrieved the same two IDs and scored 6/6.
2. **Memory-neutral tasks.** The no-memory arm scored 4/6; the two failures were the
   only memory-required tasks. Most tasks are solvable without memory → ceiling effect.
3. **No evidence utilization.** The prompt includes memory text, but nothing checks
   whether the final answer actually depended on it; a model that ignores memory and
   guesses right scores identically.
4. **No contradiction/ambiguity/abstention cases.** The categories most likely to break
   a memory system are entirely absent from the agent harness.
5. **Lexical-only via CLI.** `beacon memory list` exposes no semantic/ANN flags (verified:
   no such flags in `cli/beacon/cmd`), so the agent harness can only compare the *lexical*
   path; hashed/neural are API-only. Any agent comparison of semantic retrieval must seed
   and query in-process (Go), not through the CLI.

## 3. Scoring functions that are trustworthy

- **Functional success** (`run_tests`, exit code 0) — yes; blind to expected answer.
- **Retrieval metrics** (`p3Metrics`: R@1/5/10, MRR, nDCG@5) — yes; labels are the
  fixture's gold set.
- **Independent baselines** — yes; same corpus/queries/labels.
- **`validate_harness.py` properties** — yes; negative control proves scorer sensitivity.

## 4. Redundancy / duplication found

- Phase 4A `retrieval_baselines.py` BM25 and V6 hashed both land at R@5 0.875 on the
  locked set — the hashed arm behaves like a bag-of-words and adds no quality over BM25
  (and is beaten by FTS5 at 0.917). Reporting hashed as a "semantic" win duplicates BM25.
- The six-task fixture and the 17-case Phase 1 set both measured memory benefit, but the
  six-task set is memory-neutral; the two should not be pooled.

## 5. Do original and V6 receive equivalent external inputs?

Yes at the CLI level: same seed tool, same `memory list --query`, same project, same
limit. But because only two memories exist per task, "equivalent" degenerates to
"identical". The benchmark must seed **many** distractors so the rankers actually differ.

## 6. Is task success measured independently of retrieval recall?

Yes — success is the unittest outcome, not recall. That is correct, but it means success
alone cannot attribute *why* a system won. Phase 4B therefore records retrieval recall,
evidence-in-context, evidence-utilization, and final correctness as **separate** signals.

## 7. Audit conclusion

The existing harness is trustworthy but **cannot compare memory systems**: it seeds too
few memories, uses memory-neutral tasks, exposes only the lexical CLI path, and never
checks evidence use. Phase 4B must build a new corpus + harness with:
many distractors per case, single-gold memory-required answers, deterministic grading,
in-process retrieval for all methods, abstention cases, and explicit evidence-utilization.
