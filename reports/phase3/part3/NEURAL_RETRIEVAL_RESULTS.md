# Phase 3C — Neural Retrieval Results

**Status: BLOCKED. No real neural model was run; none is claimed.**

## 1. Access checks (2026-10-23)

| Check | Result |
|---|---|
| Apple M5 `192.0.2.4` | `ping` → Destination Host Unreachable; `ssh` → No route to host |
| `python3 -c "import torch, sentence_transformers"` | `ModuleNotFoundError` (neither installed) |
| `BAAI/bge-small-en-v1.5` in `~/.cache/huggingface` | absent |
| Local HF hub models | `SmolLM2-135M`, `Qwen2.5-0.5B`, `gpt-oss-20b`, `opt-125m` (no sentence encoder) |
| Windows HF cache | `Qwen2.5-0.5B`, `datasets--wikitext` (no BGE) |

## 2. Exact missing prerequisite

One of:
- the Apple M5 reachable on the LAN running `embed_sidecar.py` with the cached
  `BAAI/bge-small-en-v1.5` (MPS), exposed at `BEACON_EMBED_URL`; or
- a local `torch` + `sentence-transformers` runtime on WSL2 with the
  `BAAI/bge-small-en-v1.5` model already cached in `HF_HOME`.

No model was downloaded and no substantial dependency was installed (out of scope).

## 3. Fixture and harness (unchanged, ready)

- Fixture: `reports/phase3/heldout/retrieval_context_v1.json`, hash
  `FADF313E35F7E41FBC21CFB180FE88075EAC6485B6FAD851AA2CBD8F9503114A` (re-verified
  `LOCK OK`; not modified).
- Harness: `phase3a_semantic_test.go` runs the neural arm automatically when
  `BEACON_EMBED_URL` is set, printing
  `P3A_RETRIEVAL method=neural status=BLOCKED` otherwise. It labels lexical,
  hashed-semantic, hybrid-RRF, hybrid-ANN, and neural-BGE separately; hashed embeddings
  are never reported as neural.

On the Mac:
```
# terminal 1
BEACON_EMBED_MODEL=BAAI/bge-small-en-v1.5 python3 cli/beacon/internal/learning/embed_sidecar.py
# terminal 2
cd cli/beacon
BEACON_EMBED_URL=http://127.0.0.1:8077 BEACON_PHASE3_RETRIEVAL=1 \
  go test ./internal/learning/ -run TestPhase3ARetrieval -v -count=1
```

## 4. Unchanged reference numbers

No new retrieval numbers were produced in Phase 3C (no code path changed for retrieval).
The Phase 3A locked-fixture results stand: lexical R@5 0.167, hashed-semantic R@5 0.875,
hybrid RRF R@5 0.875, neural BLOCKED. Neural, ANN, and hybrid remain opt-in.

## 5. Recommendation

Run the neural arm on the Mac before any neural/hybrid default decision. This is a Phase
3D prerequisite; the six-task paired agent evaluation is separate.
