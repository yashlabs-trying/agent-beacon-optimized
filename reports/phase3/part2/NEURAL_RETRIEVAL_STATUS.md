# Phase 3B — Neural Retrieval Status

**Status: BLOCKED. No real neural model run was performed; none is claimed.**

## 1. Access checks (2026-10-23)

| Check | Result |
|---|---|
| Apple M5 `192.0.2.4` | `ping` → Destination Host Unreachable; `ssh` → No route to host |
| `<host>` | name does not resolve |
| `python3 -c "import torch, sentence_transformers"` | `ModuleNotFoundError` (neither installed) |
| `BAAI/bge-small-en-v1.5` in `~/.cache/huggingface` | absent |
| Local HF hub models | `HuggingFaceTB--SmolLM2-135M`, `Qwen--Qwen2.5-0.5B`, `openai--gpt-oss-20b`, `facebook--opt-125m` (none is a sentence encoder) |
| Windows HF cache | `Qwen--Qwen2.5-0.5B`, `datasets--wikitext` (no BGE) |

## 2. Exact missing prerequisite

To run the real-neural arm, one of the following is required:

- the Apple M5 reachable on the LAN with the `embed_sidecar.py` process running the
  cached `BAAI/bge-small-en-v1.5` (MPS), reachable at `BEACON_EMBED_URL`; **or**
- a local `torch` + `sentence-transformers` runtime on WSL2 with the
  `BAAI/bge-small-en-v1.5` model already cached in `HF_HOME`.

No model was downloaded and no substantial dependency was installed (out of scope).

## 3. What is runnable, and how to run it

The Phase 3A harness (`phase3a_semantic_test.go`) is ready and will execute the neural
arm automatically when `BEACON_EMBED_URL` points at a live sidecar; otherwise it prints
`P3A_RETRIEVAL method=neural status=BLOCKED`. On the Mac:

```
# terminal 1
BEACON_EMBED_MODEL=BAAI/bge-small-en-v1.5 python3 cli/beacon/internal/learning/embed_sidecar.py
# terminal 2
cd cli/beacon
BEACON_EMBED_URL=http://127.0.0.1:8077 BEACON_PHASE3_RETRIEVAL=1 \
  go test ./internal/learning/ -run TestPhase3ARetrieval -v -count=1
```

The harness distinguishes `lexical`, `semantic_hashed`, `hybrid_rrf`, `hybrid_ann`, and
`neural_bge`; the hashed embedder is never reported as neural.

## 4. Phase 3A reference (unchanged)

For context, the Phase 3A lexical/hashed/hybrid results on the locked 48-query fixture
remain the only measured retrieval numbers (neural BLOCKED): lexical R@5 0.167,
hashed-semantic R@5 0.875, hybrid RRF R@5 0.875. Neural/hybrid/ANN stay opt-in.

## 5. Recommendation

Run the neural arm on the Mac before any neural or hybrid default decision. This is a
Phase 3C prerequisite; the six-task paired agent evaluation is separate.
