# Beacon V7 hardening — Phase B: Intelligence and Evidence Correctness

Status: **Phase B implemented, tested, committed.**

Branch: `codex/beacon-v7-hardening`
Parent (Phase A): `e1a2ada6531cc9f5c4748050c0d5fe33e7ffa0cb`
Source under change: `cli/beacon/internal/learning/search_question.go`

## 1. Problem (discovered from the preserved uncommitted Phase B work)

An uncommitted Phase B test — `TestSearchQuestionWhatFormUsesRequestedAttribute`
(`search_question_test.go`) — was found in the worktree and preserved. It failed
against the Phase A code:

```
question "What is the environment for the orion service?" selected [owner environment]
```

**Root cause.** `analyzeQuestion` resolved the requested attribute (the "slot") by
scanning every word after the interrogative and keeping the **last** content word.
For "What is the environment for the orion service?" the last content word is
`service` (the question's *subject*), not `environment` (the requested *attribute*).
With `slot = service`, the unrelated "release owner" memory matched on the subject
and was wrongly selected as direct evidence. This is a general question-understanding
defect, not a fixture-specific one: any "what is the <attribute> for the <subject>"
question resolved to the subject.

## 2. Fix (smallest general change)

`search_question.go`:

1. All interrogatives (`which`, `where`, `when`, `what`, `how`) now resolve the slot
   to the **first content noun after the interrogative**, via a new helper
   `firstContentWord`.
2. Added `questionVerbNoise` (linking/modal/light verbs: `is/are/was/were/been/
   being/has/have/had/can/could/will/would/shall/may/might/did/get/got/many/much/
   long/kind/type`) so copulas and modals are skipped rather than mistaken for the
   requested attribute.

This is a generalized algorithm change (a question-parsing rule), not
benchmark-specific string matching.

## 3. Regression protection (development-only cases)

The preserved test already encodes the required coverage and now passes:

- **Positive:** "What is the environment for the orion service?" and "What
  environment should the orion service use for deployment?" → select only the
  deployment-environment fact.
- **Negative near-miss:** a "release owner" memory is not accepted as the environment.
- **Qualified hint:** "…environment is often canary" is not promoted to a definitive
  answer (`definitive()` rejects uncertainty words).
- **Supersession:** a superseded "was production" note is dropped.
- **Scope:** an identical fact in another project is excluded.

Existing Phase A/B tests continue to pass (all 12 `TestSearchQuestion*` tests).

## 4. Measured result

| Check | Before | After |
|---|---|---|
| `TestSearchQuestionWhatFormUsesRequestedAttribute` | FAIL (`[owner environment]`) | **PASS** (`[environment]`) |
| All `TestSearchQuestion*` (12 tests) | 11 pass / 1 fail | **12/12 pass** |
| Full learning suite | pass | **ok 52.8 s** |
| `go vet ./internal/learning/` | clean | clean |
| `go build ./...` | pass | pass |

No change to default behaviour: `SearchQuestion` remains opt-in
(`memory list --question-aware`); `ListMemories` is untouched.

## 5. Remaining limitations (carried forward)

- Question understanding is rule-based (interrogative + first content noun +
  aliases). Multi-clause questions, conjunctions, and pronoun references are not
  handled; `slotAliases` is minimal (`environment → target/destination`).
- Evidence sufficiency remains coverage/supersession/conflict based; it is not
  semantic.
- The 56-task pilot is historical development data (seven repeated families) and is
  **not** used as fresh validation here.
- Phases C–J (performance, concurrency, security/provenance, lifecycle, ANN,
  full testing, ablations, release gates) are **not yet done**.

## 6. Commit

Committed on `codex/beacon-v7-hardening` (Phase B). Original, V6-integration, Phase 5,
and V7-dev refs untouched.
