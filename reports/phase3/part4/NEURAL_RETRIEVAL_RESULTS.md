# Phase 3D — Neural Retrieval Results (Evaluation A)

**Status: BLOCKED. No real BGE-small model was run; none is claimed.**

## 1. Prerequisite checks (2026-10-23)

| Check | Result |
|---|---|
| Apple M5 `192.0.2.4` | `ping` → Destination Host Unreachable; `ssh` → No route to host |
| `sentence_transformers` | **MISSING** (`ModuleNotFoundError`) |
| `torch` | present: `2.14.0+cpu` |
| `transformers` | present: `5.17.0` |
| `BAAI/bge-small-en-v1.5` in `~/.cache/huggingface` | **absent** |
| Local HF hub models | `SmolLM2-135M`, `Qwen2.5-0.5B`, `facebook/opt-125m` (no weights), `openai/gpt-oss-20b` (config only, no weights) |
| Windows HF cache | `Qwen2.5-0.5B`, `datasets--wikitext` |
| Network to huggingface.co | reachable (HTTP 200) |

## 2. Exact blocking prerequisite

A real neural run requires **both**:

1. the `BAAI/bge-small-en-v1.5` model weights available locally (not cached here), **and**
2. a `sentence-transformers` runtime (missing), or the equivalent encoder pipeline built on the present `torch`/`transformers`.

Downloading the model is **not permitted** by the Phase 3D brief ("do not download large models … when they are not already available"). Therefore the run is BLOCKED rather than substituted. `torch`/`transformers` being present is insufficient without the model weights.

Alternative: the Apple M5 sidecar at `BEACON_EMBED_URL` — unreachable.

## 3. Fixture and harness (unchanged, ready)

- Fixture: `reports/phase3/heldout/retrieval_context_v1.json`, SHA-256
  `FADF313E35F7E41FBC21CFB180FE88075EAC6485B6FAD851AA2CBD8F9503114A` (re-verified `LOCK OK`; byte-for-byte unchanged).
- Harness: `cli/beacon/internal/learning/phase3a_semantic_test.go` runs the neural arm
  automatically when `BEACON_EMBED_URL` is set and prints
  `P3A_RETRIEVAL method=neural status=BLOCKED` otherwise. It labels lexical,
  hashed-semantic, hybrid-RRF, hybrid-ANN, and neural-BGE separately; hashed embeddings
  are never reported as neural.

Run command (on the Mac, with the sidecar):
```
# terminal 1
BEACON_EMBED_MODEL=BAAI/bge-small-en-v1.5 python3 cli/beacon/internal/learning/embed_sidecar.py
# terminal 2
cd cli/beacon
BEACON_EMBED_URL=http://127.0.0.1:8077 BEACON_PHASE3_RETRIEVAL=1 \
  go test ./internal/learning/ -run TestPhase3ARetrieval -v -count=1
```

## 4. Preserved reference numbers (not re-run, not merged)

The only measured retrieval numbers remain the Phase 3A locked-fixture results (same
fixture, same protocol): lexical R@5 0.167, hashed-semantic R@5 0.875, hybrid-RRF R@5
0.875, neural BLOCKED. No new or substituted numbers are reported.

## 5. Recommendation

Provision the BGE-small weights + `sentence-transformers` (or a reachable Mac sidecar)
and run the unchanged harness before any neural/hybrid default decision. This remains a
prerequisite for a future phase; it does not block the agent evaluation.
