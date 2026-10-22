# Phase 1 — Neural Small-Scale

## Status

**CONDITIONAL PASS** — validated on the GPU pod (V6); the Mac was unreachable, so
Phase 1 could not re-run it on Apple Silicon.

## Mac blocker

`ssh user@192.168.1.10` →
`Permission denied (publickey,password,keyboard-interactive)`. Neither
`id_ed25519` nor `runpod_kernellab` is authorized. No neural inference was run on
the Mac.

## Measured (GPU pod, bge-small-en-v1.5, revision in V6)

Interface: `HTTPEmbedder` + `embed_sidecar.py` (sentence-transformers, CUDA,
offline after model cache). Verified: model loads, real inference, dim 384,
L2-normalized, batch works, model id recorded, incremental updates, fallback when
no embedder.

| method | recall@1 | recall@5 | MRR | nDCG@5 |
|---|---|---|---|---|
| lexical | 0.118 | 0.412 | 0.235 | 0.280 |
| hashed | 0.412 | 0.706 | 0.549 | 0.590 |
| **neural (bge-small)** | **0.471** | **1.000** | **0.676** | **0.758** |

- Embedding throughput: ~1,700/s (0.59 ms/text at batch ≥1024); 504 MiB VRAM.
- The `synonym` category (0 for lexical/hashed) is the source of the neural gain.

## Limitations

- Held-out set is 17 cases, not the 30–50 target; confidence is low.
- "Neural superiority" is claimed only for this small set, not universally.
- Mac/macOS neural inference and macOS build: **NOT TESTED**.
- Empty/invalid-input handling: not separately exercised.

## Reproduction

```bash
# on a machine with the sidecar running:
BEACON_EMBED_URL=http://127.0.0.1:8077 \
  go test -run TestNeuralVsHashedQuality -v ./internal/learning/
```
