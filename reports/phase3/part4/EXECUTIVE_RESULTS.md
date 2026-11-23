# Beacon V6 — Phase 3D Executive Results

Evaluation-first phase: (A) real BGE-small neural retrieval, then (B) the six-task
paired agent evaluation. No product features, no default changes.

- Date: 2026-10-23 (Asia/Kolkata)
- Branch: `phase3d/beacon-v6-verification` (isolated worktree `/home/<user>/work/phase3d`)
- Parent: Phase 3C `d12e413a08a20d1f2abbe80a91daa1386e5edfd6`
- Ancestry verified: `99c70908` → `209a987b` → `c9275d84` → `c8a6749b` → `17a10b0f` → `d12e413a` → Phase 3D
- Frozen refs unchanged: integration `209a987bc96c29b5e5d811a794d0357f20214463`,
  prep `c9275d84185fc9d204429f913d516ff941d76be3`, baseline
  `99c709081ad49da1c5ade205929b61b19360ed9b`
- Host: WSL2 (`lappiiii`), 8 vCPU, 7.6 GiB, Go 1.27.3 linux/amd64,
  `torch 2.14.0+cpu`, `transformers 5.17.0`

## 1. Gate table

| # | Gate | Result |
|---|---|---|
| A1 | Neural prerequisites checked (Mac, model, runtime) | **PASS** |
| A2 | Real BGE-small neural retrieval run | **BLOCKED** (no local BGE weights; `sentence_transformers` missing; download not permitted) |
| B1 | Six-task paired harness built | **PASS** |
| B2 | Harness + scoring validated deterministically (reference 6/6, noop 0/6) | **PASS** |
| B3 | Model-dependent three-arm run | **BLOCKED** (no protocol-compatible local model) |
| B4 | Paired V6-vs-original agent outcomes | **NOT RUN** |
| P1 | Frozen refs / fixtures preserved | **PASS** |
| P2 | Opt-in defaults and safety decisions preserved | **PASS** |
| V1 | `gofmt` / `go build ./...` / `go vet ./...` | **PASS** |
| V2 | Full learning package suite | **PASS** (40 s) |
| V3 | Fixture hashes re-verified after evaluations | **PASS** (`LOCK OK` ×3) |

## 2. Evaluation A — neural retrieval: BLOCKED

The Apple M5 is unreachable; `sentence_transformers` is missing; and the
`BAAI/bge-small-en-v1.5` weights are not cached. `torch`/`transformers` are present but
insufficient without the model. Downloading the model is not permitted, so the gate is
**BLOCKED**, not substituted. Exact prerequisite and the ready-to-run command are in
`NEURAL_RETRIEVAL_RESULTS.md`. The Phase 3A lexical/hashed/hybrid numbers stand unchanged;
neural/hybrid remain opt-in.

## 3. Evaluation B — paired agent evaluation: BLOCKED (harness validated)

No protocol-compatible local model exists (no Ollama/vLLM/llama.cpp; no coder model
weights; the only cached LMs are `Qwen2.5-0.5B` base and `SmolLM2-135M`, and a direct
probe of the former produced degenerate non-tool-call output). The model-dependent run is
**BLOCKED**; no agent outcome is fabricated.

The harness was still built and validated, satisfying the brief's instruction to validate
the task harness and scoring rubric before observing outcomes:

- Built `reports/phase3/part4/harness/agent_eval.py` (three arms; tests kept outside the
  agent workspace; access-controlled tools; paired scoring) and
  `cli/beacon/internal/learning/testdata/phase3dseed/main.go` (seeds each task's approved
  and distractor notes).
- Built the arm binaries: `beacon-original` (`99c70908`) and `beacon-v6` (`d12e413a`).
- Validation (`--backend scripted`): reference solution → **6/6 all arms**; noop → **0/6
  all arms**; memory arms retrieved `[<task>-distractor, <task>-current]`, no-memory
  retrieved none. These are **validation runs, not model outcomes**.

The prior Phase 1 three-task Mac pilot (different fixture) reported no-memory 1/3,
original 2/3, V6 2/3 — no V6 advantage — and is retained only as context.

## 4. What the evidence supports

- **Real neural retrieval: unverified** — no BGE-small run occurred.
- **V6 vs original agent success: undetermined** — no paired model outcomes exist. V6 is
  **not** claimed to improve agent task success.
- **Harness readiness: demonstrated** — the six-task protocol, isolation, memory
  seeding/retrieval, and objective scoring all work end to end with both CLIs.

## 5. Changes and validation

Harness-only additions (no production code, no defaults changed):
- `reports/phase3/part4/harness/agent_eval.py`
- `cli/beacon/internal/learning/testdata/phase3dseed/main.go` (testdata; excluded from
  shipping builds)
- Reports under `reports/phase3/part4/`.

| Check | Result | Time |
|---|---|---|
| `gofmt -l .` | clean | — |
| `go build ./...` / `go vet ./...` | pass | — |
| `go test ./internal/learning/` | pass | 40.0 s |
| Harness validation (scripted reference + noop) | pass | < 5 s |
| Fixture hashes re-verified | `LOCK OK` ×3 | — |

## 6. Remaining risks and recommended next step

1. **Neural retrieval** requires the BGE-small weights + a sentence-transformers runtime
   (or a reachable Mac sidecar).
2. **Agent evaluation** requires a locally served coder model (e.g. Ollama
   `qwen2.5-coder:7b`) or a reachable Mac.
3. Six tasks validate the harness only; a V6-vs-original advantage claim would need ≥30
   locked tasks and a three-seed comparison (per the Phase 3 plan).
4. Keep neural, ANN, hybrid, token budgeting, and cross-kind contradiction detection
   **opt-in**; the 16-client write p99 limitation and the observed contradiction fixture
   (regression-only) are unchanged.

**Next evidence-backed recommendation:** provision the two local prerequisites and run
the unchanged harnesses; do not claim a V6 advantage until paired model outcomes
demonstrate it.
