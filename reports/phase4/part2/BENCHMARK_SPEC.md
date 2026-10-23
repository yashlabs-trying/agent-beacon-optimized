# Phase 4B — Benchmark Specification

Adversarial memory benchmark comparing original Agent Beacon with Beacon V6. The goal
is **not** to make V6 win; it is to find where each system fails and why.

## 1. Design principles

- Same agent model, system prompt, task text, context budget, and grading for all arms.
- Memory systems differ only in their retrieval; the model is held constant.
- Deterministic grading on a single gold value / required evidence / abstention.
- Many plausible distractors per case so rankers actually diverge.
- Paired by task; logical tasks (not seeds) are the primary unit.
- Freeze and hash fixtures before any held-out evaluation.

## 2. Arms

| Arm | Memory | Retrieval |
|---|---|---|
| A `none` | none | — |
| B `original` | original Beacon `99c70908` | lexical (CLI parity) |
| C `v6_lexical` | V6 `e032ee73` | `ListMemories` lexical |
| D `v6_hashed` | V6 | `SearchHybrid` hashed-semantic |
| E `v6_hybrid` | V6 | `SearchHybrid` RRF (lexical+hashed) |
| F `v6_neural` | V6 | `SearchHybrid` real BGE-small |
| G `oracle` | gold required-evidence only | diagnostic upper bound |

## 3. Case schema

```json
{
  "id": "contra_atlas_db_current",
  "category": "contradiction",
  "difficulty": "extreme",
  "project": "p4b-atlas",
  "memories": [{"id": "...", "title": "...", "body": "...", "kind": "decision", "ts": "2026-01-01"}],
  "query": "Which database is Atlas currently running?",
  "answer_type": "value|set|abstain|source",
  "gold_tokens": ["postgresql"],
  "forbidden_tokens": ["mysql", "mongodb"],
  "required_evidence": ["atlas-db-day12"],
  "expected_failure_modes": ["uses stale proposal", "trusts untrusted doc", "abstains wrongly"]
}
```

## 4. Grading (deterministic)

- Normalize the final answer: lowercase, collapse whitespace.
- `value`: PASS iff every `gold_token` appears and **no** `forbidden_token` appears.
- `set`: PASS iff all `gold_tokens` appear and no `forbidden_token` appears.
- `abstain`: PASS iff an abstention phrase appears (`cannot determine`, `not enough`,
  `unclear`, `unknown`, `insufficient`, `no information`) and no `forbidden_token` appears.
- `source`: PASS iff the gold value appears and a `source_token` (the required note's
  title) appears — i.e. the answer is *attributed*.
- A wrong-but-confident answer containing a `forbidden_token` is a **false acceptance**.

## 5. Pipeline measurements per (case, arm)

Ingestion success; required-evidence Recall@K; evidence sufficiency; evidence surviving
context construction; correct evidence utilization; final answer correctness; supported-
answer rate; correct abstention; retrieval latency; end-to-end latency; tokens; and
failure classification (ingestion / missing evidence / ranking / stale / trust /
truncation / reasoning / scoring / infra).

## 6. Difficulty mix (target for the full 1,200)

10% easy, 20% medium, 40% hard, 30% extreme. Headline comparisons emphasize hard+extreme
without hiding normal-workload performance.

## 7. Categories (12 × 100 = 1,200 logical cases)

contradiction, ambiguity/abstention, temporal, multi-hop, selection, adversarial security,
distractor resistance, evidence attribution, reliability/recovery, context efficiency,
cross-session consistency, generalization.

## 8. Scale/concurrency/fault matrix (separate from answer quality)

Stored memories 1K/10K/100K/1M; clients 1/8/16/32/64; history 10/100/1K/10K updates;
context budget 256/512/1024/2048; 3 seeds (5 for sensitive); fault injection with
deterministic seeds. 1M runs gated on measured RAM/disk first.

## 9. Victory thresholds (predefined, ambitious — not industry standards)

Task success: V6 ≥ original +10 pts on hard memory-dependent tasks. Contradiction:
≥95% P and ≥95% R. Abstention: ≥95% correct with low unnecessary refusal. Evidence:
≥95% of accepted answers supported. Reliability: zero acknowledged-write loss. Performance:
no unacceptable p99/cost regression. All reported with CIs and exact denominators.

## 10. Published benchmarks (Stage E+)

LongMemEval, LoCoMo, MemoryAgentBench — official datasets and scoring preserved; versions,
licenses, judge config, and deviations reported; never mixed with custom scores. **Not run
in this phase** (no dataset present locally; downloading is not authorized here).

## 11. Staging

A audit/spec → B corpus+validation → C dev pilot → D freeze → E held-out → F scale/fault →
G reports. A failed stage is preserved as FAILED/BLOCKED; no silent substitution.

## 12. This phase's scope

Stage A + a **small development calibration pilot** (Stage B/C) to prove the methodology
separates systems, before spending resources on 1,200 cases. The full corpus and held-out
run are specified but marked `NOT RUN` / `BLOCKED` with measured requirements.
