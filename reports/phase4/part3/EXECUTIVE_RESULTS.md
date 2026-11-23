# Beacon V6 — Phase 4C Executive Results

Intelligent memory: a trust-aware, supersession-aware context-selection layer, with
ablation and held-out evaluation.

- Date: 2026-10-23 (Asia/Kolkata)
- Branch: `phase4c/beacon-intelligent-memory` (worktree `/home/<user>/work/phase4c`)
- Base: Phase 4B `2932e8e503abff7b941e72fd7ae0c09a165f4b2e`
- Frozen refs unchanged: original `99c70908…`, Phase 3D `bcf3a1fd…`, Phase 4A `e032ee73…`, Phase 4B `2932e8e5…`
- Hosts: WSL2 laptop (8 vCPU, 7.6 GiB, Go 1.27.3) + Apple M5 Mac (`192.0.2.10`, MPS, Ollama 0.40.6)
- No paid compute, no downloads, no push/PR.

**Bottom line.** The new opt-in context-selection layer **significantly improves agent
task success** over raw semantic retrieval on the development set (26/35 vs 20/35;
paired 6–0–0, exact McNemar **p=0.031**) and reaches **oracle parity** (27/35; p=1.0). It
**eliminates the prompt-injection defect** from Phase 4B (adversarial cases 2/6 → 6/6).
On the frozen held-out set the same direction holds (14/18 vs 10/18; 4–0, p=0.125 — not
significant at n=18) and reaches oracle parity (15/18; p=1.0). The largest contributors
are **trust filtering** and **injection defense**; supersession and conflict handling
matter only in combination. Two honest negatives: memory **hurts** on unresolvable-ambiguity
cases, and the layer does not fix model reasoning failures.

---

## 1. Deliverables and gates

| Item | Status |
|---|---|
| Context-selection layer implemented (opt-in) + unit tests | **PASS** (7 unit tests) |
| Dev corpus built + validated (35 cases) | **PASS** (`7CF4EC52…` → frozen `511B4C30…`) |
| Held-out corpus built, validated, frozen (18 cases) | **PASS** (`6461AF90…`) |
| Calibration + ablations (9 arms, 315 runs) | **PASS** |
| Held-out comparison (5 arms, 90 runs) | **PASS** |
| Memory-history reliability (10/50/100 sessions) | **PASS** |
| Reliability regression (full + race suite) | **PASS** (40.2 s / 15.7 s) |
| Scale 1K/10K/100K | **PASS** (reused Phase 4A) |
| Scale 1M, 32/64-client concurrency, context-budget matrix | **BLOCKED** |
| Published benchmarks (LongMemEval/LoCoMo/MemoryAgentBench) | **NOT RUN** |
| Prompt-injection safety (dev + held-out) | **PASS** with the layer / **FAIL** without |

## 2. Headline numbers

### Development (35 cases, 9 arms, 315 runs, qwen2.5-coder:7b)

| Arm | Correct | Accuracy | 95% CI |
|---|---:|---:|---|
| none | 6/35 | 0.171 | [0.081, 0.327] |
| original Beacon (lexical) | 6/35 | 0.171 | [0.081, 0.327] |
| v6_semantic (raw retrieval) | 20/35 | 0.571 | [0.409, 0.720] |
| **v6_intelligent (+all mechanisms)** | **26/35** | **0.743** | [0.579, 0.858] |
| oracle | 27/35 | 0.771 | [0.610, 0.879] |
| ab_trust | 25/35 | 0.714 | [0.549, 0.837] |
| ab_injection | 24/35 | 0.686 | [0.520, 0.814] |
| ab_supersession | 20/35 | 0.571 | [0.409, 0.720] |
| ab_conflict | 20/35 | 0.571 | [0.409, 0.720] |

Paired: v6_intelligent vs v6_semantic **6–0–29, p=0.031**; vs original **21–1–13,
p<0.0001**; vs oracle **0–1–34, p=1.0**.

### Held-out (18 cases, 5 arms, 90 runs)

| Arm | Correct | Accuracy |
|---|---:|---:|
| none | 1/18 | 0.056 |
| original | 2/18 | 0.111 |
| v6_semantic | 10/18 | 0.556 |
| **v6_intelligent** | **14/18** | **0.778** |
| oracle | 15/18 | 0.833 |

Paired: v6_intelligent vs v6_semantic **4–0–14, p=0.125**; vs original **13–1–4, p=0.0018**;
vs oracle **0–1–17, p=1.0**.

### Category movement (dev, semantic → intelligent → oracle)

| Category | semantic | intelligent | oracle |
|---|---:|---:|---:|
| contradiction | 5/11 | **7/11** | 7/11 |
| adversarial | 2/6 | **6/6** | 6/6 |
| ambiguity | 7/8 | 7/8 | 8/8 |
| decision | 6/6 | 6/6 | 6/6 |
| control | 0/4 | 0/4 | 0/4 (invalid — see §5) |

## 3. What each mechanism contributes (dev)

| Mechanism added to semantic | Correct | Paired vs semantic |
|---|---:|---|
| supersession only | 20/35 | 0 / 0 (p=1.0) |
| conflict only | 20/35 | 0 / 0 (p=1.0) |
| trust only | 25/35 | +6 / −1 (p=0.125) |
| injection defense only | 24/35 | +4 / −0 (p=0.125) |
| **all** | **26/35** | **+6 / −0 (p=0.031)** |

No single mechanism equals the combination → the mechanisms are complementary.

## 4. Safety

- Prompt-injection attacks (6 dev adversarial cases): raw semantic retrieval succeeded
  against the agent in **4/6**; with injection defense, **0/6** (dev) and **0/4**
  (held-out). False-memory rejection stayed low (only `adv_mixed` under trust-only, which
  over-drops a mixed memory — a precision trade-off).
- Untrusted newer corrections no longer override verified facts (`cx_false_newer`,
  `cx_adversarial_correction` now pass).

## 5. Honest negatives

- **Memory hurts unresolvable ambiguity.** On `am_unresolvable`, none/original correctly
  abstain, but every memory arm answers a wrong value. Retrieval of a plausible-but-
  insufficient hint pushes the model to over-answer. The oracle (no evidence) abstains.
- **Model reasoning is the remaining ceiling.** `cx_cross_kind`, `cx_negation`,
  `cx_version_specific`, `cx_temporal_obsolete` fail for *all* arms including the oracle
  — these are model failures, not memory failures.
- **Controls are invalid as designed.** The note-only system prompt makes the model
  abstain on memory-independent tasks (0/4 and 0/2 for every arm including oracle). The
  controls must use a prompt that permits general knowledge; excluded from the memory
  comparison.
- **Held-out improvement is directional, not significant** (n=18, p=0.125).

## 6. Verdict per dimension

| Dimension | Verdict |
|---|---|
| Correct contradiction resolution | **CONDITIONAL** (improved 5→7/11; cross-kind/negation remain) |
| Contradiction detection (locked set) | **PASS** (P=1.0, R=0.50 default; cross-kind opt-in 1.0) |
| Correct applicable-fact selection | **PASS** (supersession/trust fixes verified on dev) |
| New-correction vs misleading-update | **PASS** (false-newer rejected) |
| Ambiguity handling | **CONDITIONAL** (resolvable yes; unresolvable regresses with memory) |
| Prompt-injection resistance | **PASS** with layer; **FAIL** without |
| Reliability over history | **PASS** (10/50/100 sessions) |
| Scale 1K/10K/100K | **PASS**; 1M **BLOCKED** |
| Overall V6 intelligent vs semantic on unseen tasks | **CONDITIONAL** (directionally better, 4–0, not significant at n=18) |

## 7. Recommendation

**Keep the layer opt-in and continue.** The mechanisms are sound and complementary; the
next phase must (1) fix the unresolvable-ambiguity over-answering, (2) add a larger
held-out (≥100 tasks, 3 seeds) to convert the directional held-out win into a significant
one, and (3) make supersession/conflict contribute independently (today they only help in
combination). Do not promote any mechanism to default yet.

---

## 8. Artifacts

- Reports: `reports/phase4/part3/*.md`
- Raw evidence: `reports/phase4/part3/RAW_EVIDENCE/`
- Harnesses/fixtures: `reports/phase4/part3/harness/`, `fixtures/`
- Layer: `cli/beacon/internal/learning/context_select.go` (+ `context_select_test.go`,
  `phase4c_select_test.go`, `phase4c_history_test.go`, `testdata/phase4cseed/`)
- Reproduction: `reports/phase4/part3/run_phase4c.sh`
