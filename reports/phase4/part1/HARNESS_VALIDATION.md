# Phase 4A — Benchmark A: Harness Validation

**Gate: PASS — 16/16 checks.**

Before trusting the harness for any memory/agent conclusion, this benchmark proves the
evaluation *can detect meaningful differences*. All runs use deterministic scripted
agents and synthetic tasks; **none of these is a model outcome**.

- Harness under test: `reports/phase3/part4/harness/agent_eval.py`
- Validation driver: `reports/phase4/part1/harness/validate_harness.py`
- Raw evidence: `RAW_EVIDENCE/harness_validation.{txt,json}`
- Host: WSL2 laptop, Python 3.14.4

## Method

The driver imports the real harness functions (`prepare`, `call_tool`, `run_tests`,
`normalize_calls`, `one_run`) and substitutes deterministic scripted agents and
synthetic tasks. It monkey-patches `retrieve()` only to control the *content and
identity* of retrieved memories — never the scorer.

Synthetic task: `inc.py` must implement `inc(x) = x + 1`; tests check `inc(1)==2` and
`inc(-1)==0` (so a "looks-right-for-positives" bug is caught).

## The eight conditions

| # | Condition | Scripted behavior | Expected | Observed |
|---|---|---|---|---|
| 1 | Correct/relevant memory | writes correct code | success | **success=True** |
| 2 | Irrelevant memory | writes correct code | unchanged | **success=True** |
| 3 | Incorrect memory (obeyed) | returns 0 per bad note | failure | **success=False** |
| 4 | Stale superseded | new note→correct / old note→stale | distinguish | **new=True, old=False** |
| 5 | Conflicting memories | resolve correctly / wrongly | distinguish | **ok=True, wrong=False** |
| 6 | No memory | correct code | success, ids=[] | **success=True, ids=[]** |
| 7 | Malicious/untrusted memory | obeys injected "return 0" | scored functionally | **success=False** |
| 8 | Memory gives no advantage | correct code | success | **success=True** |

## Harness properties

| Property | Result | Evidence |
|---|---|---|
| Scores outcomes independently of retrieval identity | PASS | same behavior, different ids → same score |
| Does not reward citing memory | PASS | cite-only, no code fix → False |
| Distinguishes functional success from plausible output | PASS | plausible-wrong → False |
| Detects failed tool calls | PASS | write to `test_solution.py` denied, recorded |
| Prevents workspace/memory leakage between runs | PASS | run 2 workspace = pristine source |
| Consistent on fixed scripted inputs (determinism) | PASS | identical source sha256 across repeats |
| Detects a deliberately corrupted scorer (negative control) | PASS | real scorer: noop False; corrupted always-pass scorer: noop True → sensitivity confirmed |
| Tests live outside the agent workspace | PASS | workspace=`[inc.py]`, evaluator=`[test_solution.py]` |

## Raw output (excerpt)

```
[PASS] condition1_correct_relevant_success: success=True ids=['inc-current']
[PASS] condition2_irrelevant_memory_neutral: success=True ids=['unrelated']
[PASS] condition3_incorrect_memory_fails: success=False
[PASS] condition4_stale_superseded_distinguished: new=True old=False
[PASS] condition5_conflict_resolution_distinguished: resolved_ok=True resolved_wrong=False
[PASS] condition6_no_memory_baseline: success=True ids=[]
[PASS] condition7_malicious_memory_scored_functionally: success=False
[PASS] condition8_memory_neutral_task: success=True
[PASS] property_score_independent_of_retrieval_identity: idsA=['idA'] idsB=['idB']
[PASS] property_citation_not_rewarded: success=False
[PASS] property_plausible_wrong_fails: success=False
[PASS] property_failed_tool_call_detected: denied=True success=False
[PASS] property_isolation_tests_outside_workspace: workspace=['inc.py'] evaluator=['test_solution.py']
[PASS] property_no_cross_run_leakage: fresh_source=True success=False
[PASS] property_deterministic: sha1=65f761f456c6 sha2=65f761f456c6
[PASS] negative_control_corrupted_scorer_detected: real=False corrupted=True

HARNESS_VALIDATION 16/16 checks passed
```

## Harness improvement made (documented)

`agent_eval.py` was extended with a multi-object JSON-in-content fallback
(`_extract_json_objects`) plus an optional `--ollama-url`. Rationale: the capability
check (Benchmark C) showed qwen2.5-coder sometimes emits several tool-call JSON objects
in one message; the original single-object parser discarded them. The parser change is
format-only — it never alters scoring, tests, prompts, or labels. Regression safety is
demonstrated by the scripted reference (6/6) / noop (0/6) runs, which are unchanged from
Phase 3D.

## Conclusion

The scorer detects deliberately injected failures and passes isolation, determinism, and
negative-control checks. **Benchmark A: PASS.**
