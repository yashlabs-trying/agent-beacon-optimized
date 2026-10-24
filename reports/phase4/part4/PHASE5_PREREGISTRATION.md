# Phase 5 — Pre-registration Plan (300-Task Campaign)

**Status: NOT RUN in Phase 4D.** This is the frozen protocol for a separate Phase 5. The
Phase 4D implementation and configuration must be frozen before Phase 5 begins; the 300
tasks are not inspected or tuned on here.

## 1. Suites (3 × 100 unique tasks = 300)

| Suite | Focus | Examples |
|---|---|---|
| 1. Long-term memory & complex engineering | multi-session knowledge, debugging, prior failed attempts, architecture decisions, long-horizon coding | reuse a fix; avoid a known-bad approach; apply a project convention |
| 2. Contradiction, ambiguity & intelligent decisions | correct applicable facts, trustworthy corrections, stale knowledge, multi-hop, uncertainty | resolve a correction chain; abstain when evidence is missing |
| 3. Adversarial reliability & memory at scale | untrusted evidence, prompt injection, changing history, project isolation, large corpora, budgeting | reject a forged correction; ignore an injected instruction |

The three suites contain **distinct** tasks (not relabelled copies). External datasets
(LongMemEval/LoCoMo/MemoryAgentBench) are used only with their official protocol and
labelled as such; otherwise the suites are labelled custom.

## 2. Arms

| Arm | Configuration |
|---|---|
| A | No memory |
| B | Original Beacon `99c70908` |
| C | V6 Semantic (frozen Phase 4B config) |
| D | V6 Intelligent (frozen Phase 4D config) |
| E | Oracle (minimal clean evidence) |

Secondary: V6 hashed vs neural, matched budgets.

## 3. Controls

- Same model (`qwen2.5-coder:7b` primary; a second model if available), same prompt,
  tools, K, context budget, and scoring across arms.
- **Prompt fix for memory-independent tasks:** a prompt variant that permits general
  knowledge so no-memory agents can solve controls (repairs the Phase 4C/4D control defect).
- Isolated state per (task, arm); randomized arm order.

## 4. Primary and secondary endpoints

- **Primary:** paired end-to-end task success (binary, functional/objective check).
- **Safety gates (independent):** injection attack success, cross-project contamination,
  unsupported claims. A higher success score cannot compensate for a safety failure.
- **Secondary:** contradiction resolution, ambiguity handling, stale rejection, latency,
  tokens, cost per success, reliability vs history size.

## 5. Sampling, eligibility, deduplication, leakage

- 100 tasks/suite authored under a new `dataset_id`, disjoint from Phase 4C/4D fixtures.
- Eligibility: objective automated check (executable test or deterministic gold); no task
  whose answer leaks through IDs, filenames, or metadata visible to the agent.
- Deduplication: n-gram + embedding similarity < threshold against dev corpora.
- Held-out tasks are frozen and hashed **before** any model run; no tuning after outcomes.

## 6. Statistical analysis

- Unit: the logical task. Seeds (≥3) are dependent observations within a task.
- Paired comparison per task; exact McNemar / paired bootstrap with task-level resampling.
- Pre-registered primary comparison D vs C; secondary C vs B, D vs B, D vs E.
- Report exact denominators, per-category breakdown, win/loss/tie, CIs, worst-case category.

## 7. Compute estimate

- 300 tasks × 5 arms × 3 seeds = **4,500 model executions** (plus secondary arms). At the
  observed ~1–15 s/execution this is many hours; run in bounded batches, abort if limits
  are hit, and report partial results with exact blocked cells.

## 8. Freeze

Before Phase 5: freeze the Phase 4D commit, model id/hash, prompts, K, budgets, scoring,
and the hashed task fixtures. Record an evaluation manifest. No push/PR without
authorization; no paid compute.

## 9. Success criteria

Phase 5 declares V6 Intelligent superior only if D beats C on the held-out paired primary
endpoint with a CI excluding 0, **and** all safety gates pass, **and** no material
latency/cost regression. Otherwise report honestly and keep the simpler design.
