# Beacon V6 — Phase 3A Executive Results

Phase 3A (first half of Phase 3): real-neural retrieval, bounded ANN, contradiction
and trust, token budgeting, and reliability/security/concurrency verification of the
integrated V6 memory engine.

- Date: 2026-10-23 (Asia/Kolkata)
- Branch: `phase3a/beacon-v6-verification` (isolated worktree)
- Base (verified preparation state): `c9275d84185fc9d204429f913d516ff941d76be3`
- Verified integration HEAD (unchanged): `209a987bc96c29b5e5d811a794d0357f20214463`
- Original Beacon baseline (unchanged): `99c709081ad49da1c5ade205929b61b19360ed9b`

**Bottom line: an honest partial result.** ANN, token budgeting, concurrency (≤8
clients), crash durability, race, and the full local CLI suite pass. The real-neural
retrieval arm is **BLOCKED** (the Apple M5 is unreachable and no BGE model/sidecar
exists on the WSL2 host). The contradiction detector meets the precision gate but
**fails the recall gate (0.25)** on the locked 24-pair set. Neural, ANN, and hybrid
remain **opt-in**; no default was changed.

---

## 1. Environment and provenance

| Item | Value |
|---|---|
| Host | WSL2 (`lappiiii`), Linux 6.18.33.2-microsoft-standard-WSL2, x86_64 |
| CPU / RAM | 8 vCPU / 7.6 GiB (Windows 11 host, 15.7 GiB total) |
| Go | go1.27.3 linux/amd64 (`/home/<user>/tools/go/bin/go`) |
| Worktree | `/home/<user>/work/phase3a` (native ext4, not a Windows CRLF mount) |
| Baseline worktree | `/home/<user>/work/phase3a-baseline` (detached at 99c70908) |
| Mac M5 | **Unreachable** — `192.0.2.4` "No route to host", `<host>` unresolvable |
| Neural model | **Absent locally** — no `BAAI/bge-small-en-v1.5`, no `torch`/`sentence-transformers` on WSL2 |

Ancestry verified before work: `99c70908` → ancestor of `209a987b` → ancestor of
`c9275d84` (all `merge-base --is-ancestor` exit 0). The integration branch, original
baseline clone, and locked fixtures were not modified.

### Locked fixtures (hashes re-verified with `validate_lock.py`)

| Dataset | File | SHA-256 | Size |
|---|---|---|---|
| Retrieval | `retrieval_context_v1.json` | `FADF313E…114A` | 12 topics, 48 queries |
| Contradiction | `contradiction_v1.json` | `B3B4CAC0…8E84` | 24 pairs (12 conflict / 12 non) |
| Agent tasks | `agent_tasks_v1.json` | `409195B0…C530` | 6 tasks (**out of scope for 3A**) |

`python reports/phase3/heldout/validate_lock.py` → `LOCK OK` for all three, before and
after evaluation.

---

## 2. Gate summary

| # | Gate (from Phase 3 plan / Phase 3A brief) | Result | Evidence |
|---|---|---|---|
| A1 | Hashed/lexical ranking measured on 48 locked queries | **PASS** | `RAW_EVIDENCE/retrieval_v6.txt` |
| A2 | Real-neural R@5 ≥ hashed & lexical, no identifier regression, p95 < 100 ms, RSS < 1 GiB | **NOT TESTED (BLOCKED)** | no sidecar/model; `method=neural status=BLOCKED` |
| B1 | ANN Recall@10 ≥ 0.95 vs exact at bounded sizes | **PASS** (0.9953 / 1.000 / 1.000 / 0.9984) | `ann_vs_exact_m64.txt`, `ann_vs_exact_100k.txt` |
| B2 | ANN build 100K < 2 min, peak RSS < 1 GiB | **PASS** (102.25 s, 358 MB) | `ann_vs_exact_100k.txt` |
| B3 | ANN invalid/zero/non-finite/dim-mismatch rejection, determinism, persistence | **PASS** | `ann_correctness.txt`, `ann_correctness_extra.txt`, `ann_lifecycle.txt` |
| C1 | Contradiction precision ≥ 0.95 | **PASS** (1.000) | `contradiction_locked.txt` |
| C2 | Contradiction recall ≥ 0.90 | **FAIL** (0.250) | `contradiction_locked.txt` |
| C3 | Zero project/version boundary violations | **PASS** (0/24) | `contradiction_locked.txt` |
| C4 | Hostile content cannot bypass approval/trust boundary | **PASS** | `security_trust.txt` |
| D1 | Reproduce 106/144 @ 3,840 vs 109/144 @ 5,674 | **PASS** (exact) | `token_budget.txt` |
| D2 | Budget selector retains all top-five findings while saving ≥ 20 % | **FAIL** (loses 3 findings) → keep opt-in | `token_budget.txt` |
| E1 | build / vet / gofmt / `git diff --check` | **PASS** (gofmt defect fixed) | `build_vet_fmt.txt`, `gofmt_before/after.txt` |
| E2 | Zero acknowledged-write loss, no non-busy error, no cross-project leak | **PASS** | `durability_crash.txt`, `concurrency_*.txt` |
| E3 | Steady-state p99 < 250 ms at the supported client count | **CONDITIONAL** — PASS at ≤8 clients, FAIL at 16 | `concurrency_*.txt` |
| E4 | Race detector clean (learning, hnsw) | **PASS** | `race_learning_hnsw.txt` |
| E5 | Repository-wide CLI suite | **PASS** (after building the real hooks binary) | `full_cli_suite_hooksfixed.txt` |
| F1 | No neural/ANN/hybrid default enabled | **PASS** (opt-in preserved) | code inspection |

---

## 3. Baseline vs V6 (locked 48-query retrieval set)

Identical fixture, identical harness, identical relevance labels. `Original` = lexical
search compiled from `99c70908`; `V6` = the Phase 3A worktree. Hashed/hybrid arms exist
only in V6.

| Method | R@1 | R@5 | MRR | NDCG@5 | p50 µs | p95 µs |
|---|---:|---:|---:|---:|---:|---:|
| Original lexical | 0.1667 | 0.1667 | 0.1667 | 0.1344 | 544 | 983 |
| V6 lexical | 0.1667 | 0.1667 | 0.1667 | 0.1344 | 571 | 1241 |
| V6 hashed semantic | 0.6875 | 0.8750 | 0.7635 | 0.7156 | 1267 | 1979 |
| V6 hybrid RRF | 0.6875 | 0.8750 | 0.7635 | 0.7156 | 1175 | 1689 |
| V6 hybrid ANN | 0.6875 | 0.8750 | 0.7635 | 0.7156 | 1142 | 1850 |
| V6 real neural | — | — | — | — | — | — (BLOCKED) |

Per-category detail and interpretation are in `NEURAL_RETRIEVAL_RESULTS.md`. The V6
lexical path is **identical** to the original on quality (both use the same AND-substring
predicate); the difference is the indexed execution path, not ranking.

---

## 4. Defects found, fixed, and regression evidence

1. **Inherited `gofmt` defect (test-only).** `phase2_concurrency_test.go` and
   `phase2_trust_adversarial_test.go` were committed with mis-aligned `var` blocks and
   comment columns; `gofmt -l` flagged both at `c9275d84` (before any Phase 3A change).
   Classified **inherited**, not V6. Fix: applied `gofmt -w` (whitespace only). Evidence:
   `RAW_EVIDENCE/gofmt_before.txt` (2 files) vs `gofmt_after.txt` (empty).
2. **No new product defect** was found in the integrated V6 memory engine. The low
   lexical recall and the contradiction recall shortfall are **inherited behaviours**
   reproduced on the locked set, not regressions introduced by V6.
3. **Harness defect (Phase 3A tooling, not product).** The first retrieval harness
   version accumulated per-query hit *flags* instead of per-query *booleans*, corrupting
   R@1/R@5. Caught by cross-checking against a per-query debug dump and corrected before
   any number was recorded. The corrected harness is committed.

Regression tests added: `phase3a_retrieval_test.go`, `phase3a_semantic_test.go`,
`phase3a_ann_test.go`, `phase3a_contradiction_test.go` (all opt-in; no effect on the
default suite).

---

## 5. Resource usage and interrupted/skipped tests

| Workstream | Wall time | Peak RSS | Notes |
|---|---:|---:|---|
| Retrieval (48 queries × 5 methods) | < 1 s | ~30 MB | |
| ANN 1K/10K/25K | 44 s | 148 MB | M=64 |
| ANN 100K | 218 s wall (102 s build) | 358 MB | bounded, completed |
| Contradiction | < 1 s | ~10 MB | |
| Token budget | 1.5 s | ~40 MB | |
| Concurrency 1/4/8/16 | 42 s | < 30 MB heap | |
| Race (learning+hnsw) | 575 s | — | |
| Full CLI suite | 86 s | — | |

- **Skipped/blocked:** real-neural retrieval (no sidecar/model); the six-task agent
  evaluation (explicitly out of scope for Phase 3A); Mac-side MPS tests.
- **Not attempted by design:** unbounded 1M ANN build; neural/ANN default changes.
- **Stopped experiments:** none — the 100K ANN run stayed within its time/RSS budget.

---

## 6. Remaining production blockers and Phase 3B prerequisites

1. **Neural retrieval is unverified on the locked set.** Must be run on the Mac M5 with
   the cached `BAAI/bge-small-en-v1.5` sidecar before any neural/hybrid default decision.
2. **Contradiction recall 0.25 on the locked set** (cross-kind pairs and non-marker
   negations are missed). Keep advisory-only. Phase 3B must extend cross-kind detection
   and re-validate before any automatic action. Recall must not be compared across the
   65-case adversarial set (0.758) and the 24-case locked set as if comparable.
3. **16-client write tail latency (p99 848 ms)** exceeds the 250 ms target; the supported
   operational budget is **≤8 clients** (p99 132 ms). Investigate blocking index rebuilds
   and connection churn before raising it.
4. **Token budgeting loses 3/109 findings**; keep opt-in. Evidence-preserving dedup is
   promising (112/144) but not default.
5. **Low-`M` / flat-space ANN** recall collapses (flat-space R@10 = 0.000); retain M=64.
6. **Mac-side verification** (MPS neural, Apple race suite) remains untested.
7. **Phase 3B** = the six-task + expanded held-out real-agent evaluation and the final
   three-seed comparison; not performed here.
