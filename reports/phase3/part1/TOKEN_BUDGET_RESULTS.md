# Phase 3A — Token Budgeting and Evidence Retention (Workstream D)

**Status: reproduced exactly. The current budget selector does NOT meet the "retain all
top-five findings while saving ≥ 20 %" gate, so budgeting stays opt-in.**

## 1. Method

Reproduced the existing Phase 2 evaluation harness (`TestPhase2TokenSelectionStrategies`,
`TestPhase2RankingAlternatives`) unchanged on the Phase 3A worktree.

- Fixture: the Phase 2 ranking dataset built by `buildRankingDataset` (24 topics ×
  4 memories = 96 + 40 distractors = 136 memories; 96 queries, 48 tune / 48 held-out).
- **144 finding-slots** = 48 held-out queries × 3 relevant groups
  (`{gold,dup}`, `{related}`, `{related2}`).
- Strategies: `fixed_top5` (unbudgeted), `token_budget` (current default candidate),
  `evidence_dedup`, `trust_aware`, `diversity_mmr`.

## 2. Reproduced results

| Strategy | Cases | Tokens | Tokens/case | Relevant retained | Redundancy | Latency µs/case |
|---|---:|---:|---:|---:|---:|---:|
| `fixed_top5` (unbudgeted) | 48 | 5,674 | 118.2 | **109/144** | 0.569 | 0.0 |
| `token_budget` (current) | 48 | **3,840** | 80.0 | **106/144** | 0.574 | 15.7 |
| `evidence_dedup` | 48 | 5,563 | 115.9 | **112/144** | 0.525 | 3.4 |
| `trust_aware` | 48 | 5,670 | 118.1 | 109/144 | 0.581 | 239.3 |
| `diversity_mmr` | 48 | 5,674 | 118.2 | 109/144 | 0.569 | 5,633.5 |

This matches the integration report exactly: **106/144 at 3,840 tokens** (budgeted)
versus **109/144 at 5,674 tokens** (unbudgeted top-five).

## 3. Gate evaluation

Plan gate: *"A default token-saving selector must retain every distinct finding retained
by unbudgeted top-five while saving ≥ 20 % tokens; otherwise keep budgeting opt-in."*

| Criterion | Target | Result |
|---|---|---|
| Tokens saved vs top-five | ≥ 20 % | **PASS** — 3,840 vs 5,674 = 32.3 % fewer |
| Findings retained vs top-five | all 109 | **FAIL** — retains 106, **loses 3** |

**Conclusion:** the current budget selector saves 32 % of tokens but loses 3 of the 109
findings that unbudgeted top-five retains (2.8 %). It must remain **opt-in**. No ranking
policy was changed and none was made default.

## 4. Duplicate evidence and evidence-preserving dedup

- `evidence_dedup` (drop results whose joined `TraceID` set was already seen) retains the
  **most findings of any strategy (112/144)** at 5,563 tokens with the **lowest
  redundancy (0.525)** and negligible latency (3.4 µs/case). It is the best
  evidence-preserving candidate, but it saves only ~2 % tokens versus top-five, so it is
  **not** a token-saving default either.
- The budgeted strategy raises redundancy slightly (0.574 vs 0.569) because truncation
  keeps shorter, more similar bodies.

## 5. Which distinct findings are lost, and downstream impact

- The harness reports **aggregate retained counts only**; the three lost finding-slots are
  `fixed_top5 − token_budget` and are not individually identified by the committed test.
  Pinpointing them requires a per-finding diff, which this Phase 3A run did not add.
- **Downstream-conclusion impact is not testable in Phase 3A**: the six agent tasks that
  would consume this context are explicitly out of scope (Phase 3B). No claim is made that
  the 3 lost findings are harmless.

## 6. Reproduce

```
cd cli/beacon
go test ./internal/learning/ -run 'TestPhase2RankingAlternatives|TestPhase2TokenSelectionStrategies|TestContextBudgeting|TestSelectContextActuallyTruncatesBody' -v -count=1
```

Raw evidence: `RAW_EVIDENCE/token_budget.txt`.
