# Phase 4C — Ambiguity and Decision Evaluation

## Part 1 — Ambiguity (8 dev cases + 4 held-out)

The Phase 4B ambiguity cases were non-discriminating; Phase 4C rebuilt them.

| Case | type | none | semantic | intelligent | oracle | expected |
|---|---|---|---|---|---|---|
| am_resolvable | resolvable | ✗ | ✓ | ✓ | ✓ | answer (memory disambiguates) |
| am_implicit | implicit | ✗ | ✓ | ✓ | ✓ | answer (UTC constraint) |
| am_intent | intent | ✗ | ✓ | ✓ | ✓ | answer (10% default) |
| am_high_risk | high-risk | ✓ | ✓ | ✓ | ✓ | do NOT drop the table |
| am_should_not_abstain | control | ✓ | ✓ | ✓ | ✓ | answer (flag disabled) |
| am_misleading | misleading | ✗ | ✓ | ✓ | ✓ | use current branch |
| am_competing_conventions | competing | ✗ | ✓ | ✓ | ✓ | Service A prefix |
| **am_unresolvable** | unresolvable | ✓ | **✗** | **✗** | ✓ | abstain |

**Measured separately:** correct autonomous resolution (6/6 for semantic/intelligent),
appropriate abstention (`am_unresolvable`: none/original/oracle ✓), **unsafe action 0**,
**unsupported guessing 1** (the memory arms guess on `am_unresolvable`), unnecessary
abstention 0.

**Key finding (negative for memory):** on genuinely unresolvable ambiguity, *memory
hurts*. A plausible-but-insufficient retrieved hint (`amb-hint`) makes the agent answer a
wrong value where a memory-less agent abstains. The oracle (no evidence) abstains. This
is the top defect to fix next: the layer should emit an explicit "insufficient evidence"
signal that suppresses guessing.

Held-out ambiguity (4 cases): semantic 4/4, intelligent 4/4, oracle 4/4, none 1/4 — the
resolvable/misleading cases discriminate; `h_unresolvable_amb` again shows the over-answer.

## Part 2 — Decision (6 dev + 3 held-out)

| Case | none | semantic | intelligent | oracle |
|---|---|---|---|---|
| dec_architecture | ✗ | ✓ | ✓ | ✓ |
| dec_reuse_fix | ✗ | ✓ | ✓ | ✓ |
| dec_reject_obsolete | ✓ | ✓ | ✓ | ✓ |
| dec_api_version | ✗ | ✓ | ✓ | ✓ |
| dec_attractive_wrong | ✗ | ✓ | ✓ | ✓ |
| dec_missing_info | ✓ | ✓ | ✓ | ✓ |

Semantic retrieval already solves decision tasks (6/6); the layer does not regress them.
Held-out decision 3/3 for semantic/intelligent/oracle.

## Scoring (what the agent does, not what it says)

Grading is deterministic on the final answer: `value`/`set` require the gold token(s) and
forbid the wrong-memory tokens; `abstain` requires an abstention phrase and no wrong
token. A wrong-but-confident answer containing a forbidden token is a **false acceptance**.
No credit for explanation length.

## Verdict

- Resolvable / misleading / implicit / competing ambiguity: **PASS** (memory + layer).
- Unresolvable ambiguity: **FAIL** (memory over-answers) — open defect.
- Decision tasks: **PASS** (no regression).
