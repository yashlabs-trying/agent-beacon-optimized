# Phase 4C — Baseline Reproduction (Stage A)

## 1. Provenance

| Reference | SHA | Verified |
|---|---|---|
| Original Beacon | `99c709081ad49da1c5ade205929b61b19360ed9b` | commit present |
| Phase 3D | `bcf3a1fd46d5865ca21b48a224646b9e107b2f86` | commit present |
| Phase 4A | `e032ee73…` | commit present |
| Phase 4B | `2932e8e503abff7b941e72fd7ae0c09a165f4b2e` | commit present (Phase 4C base) |

Ancestry: `99c70908 → 209a987b → c9275d84 → c8a6749b → 17a10b0f → d12e413a → bcf3a1fd
→ e032ee73 → 2932e8e5 → Phase 4C`. Phase 3 locked fixtures re-verified `LOCK OK ×3`.

## 2. Reproduced Phase 4B result (subset)

Phase 4B reported, on its 20-case dev corpus: none 2/20, original 2/20, v6_lexical 2/20,
v6_hashed 15/20, v6_hybrid 15/20, v6_neural 16/20, oracle 19/20. Phase 4C did **not**
re-run that corpus (it is explicitly debug/regression-only per the brief); instead the
key structural findings were re-confirmed on the new corpus:

- `original` and `v6_lexical` remain identical on natural-language queries (the lexical
  AND-substring predicate returns `[]`; verified by running the real CLI on a seeded DB).
- Semantic retrieval finds the relevant evidence (mean required-evidence recall 1.0 in
  both phases).
- Raw semantic retrieval still co-retrieves stale/untrusted/conflicting memories
  (reproduced: `cx_direct_correction` retrieved `[to-old, to-new]`, `adv_fake_system`
  retrieved `[sys-good, sys-evil]`).

## 3. Harness audit carried forward

The Phase 4B audit conclusions still hold and shaped Phase 4C: the six-task harness
cannot compare memory systems (2 memories/task); the CLI exposes only lexical retrieval;
scoring is functional and trustworthy. Phase 4C therefore seeds many distractors, uses a
single-gold memory-required answer, and drives semantic retrieval in-process.

## 4. Environment

| Host | Spec |
|---|---|
| WSL2 laptop | Linux 6.18.33.2-microsoft-standard-WSL2, 8 vCPU, 7.6 GiB, Go 1.27.3, Python 3.14.4 |
| Apple M5 Mac | macOS 26.6.2, 16 GiB, Ollama 0.40.6 (qwen2.5-coder:7b, qwen2.5-coder:1.5b), BGE-small dim=384 (MPS) |

Model: `qwen2.5-coder:7b`, temperature 0, seed 42, num_ctx 4096, num_predict 256.
Fixtures: dev `511B4C30…`, held-out `6461AF90…`.
