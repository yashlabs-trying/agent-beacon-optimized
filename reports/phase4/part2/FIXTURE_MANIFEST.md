# Phase 4B — Fixture Manifest

## 1. Development corpus (shipped, frozen)

| Field | Value |
|---|---|
| dataset_id | `beacon-phase4b-adversarial-dev-v1` |
| file | `reports/phase4/part2/fixtures/phase4b_dev_v1.json` |
| split | **development** |
| status | frozen |
| cases | 20 |
| SHA-256 | `05853A8761851E2B06393BB358C665947B2CA1F827BB77ABDBD49BBA925210EA` |
| hash rule | SHA-256 of file bytes after CRLF→LF normalization |

By category: contradiction 4, temporal 3, ambiguity 2, multi_hop 2, selection 2,
adversarial 2, generalization 2, distractor 1, evidence 1, context 1.
By difficulty: medium 4, hard 9, extreme 7 (easy 0).

Every case carries: unique id, category, difficulty, project, timestamped memory
history, query, answer_type (`value|set|abstain|source`), `gold_tokens`,
`forbidden_tokens`, `required_evidence`, optional `source_token`, and expected failure
modes. Validation (`build_dev_corpus.py`) enforces: unique ids, gold tokens present in
required evidence, forbidden tokens absent from required evidence, required evidence
exists, abstain cases have empty gold + non-empty forbidden, and project ids unique.
The generator is deterministic; re-running reproduces the same hash.

## 2. Held-out corpus (NOT shipped — protocol)

A held-out split of ≥100 cases/category (≥1,200) must be authored under a **new**
`dataset_id` (`beacon-phase4b-adversarial-heldout-v1`), frozen and hashed **before** any
model run, and never used to tune prompts, retrieval, or thresholds. Its file is not
included here so it cannot be inspected during development.

## 3. Freeze discipline

- `validate_lock.py` covers the Phase 3 locked fixtures (retrieval/contradiction/agent).
- The Phase 4B dev hash above must be re-verified before and after any run:
  `python3 reports/phase4/part2/harness/build_dev_corpus.py --out <tmp> --manifest <tmp>`
  reproduces the hash; any change requires a new dataset_id.
- Existing Phase 3 locked fixtures were re-verified `LOCK OK ×3` at Phase 4B start.

## 4. Arms and config freeze

Arms A–G (none, original, v6_lexical, v6_hashed, v6_hybrid, v6_neural, oracle); model
`qwen2.5-coder:7b`; K=5; deterministic grading; temperature 0, seed 42. These are frozen
for any held-out comparison.
