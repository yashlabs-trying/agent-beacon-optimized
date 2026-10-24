# Phase 1 — Neural Small-Scale (22 October 2026 Mac revalidation)

The Mac is reachable by authenticated SSH. The optional `BAAI/bge-small-en-v1.5` sentence-transformers sidecar ran **locally** on Apple M5 MPS, after a small device-selection repair (CUDA, then MPS, then CPU). The cached model revision was `5c38ec7c405ec4b44b94cc5a9bb96e735b38267a`; its model card states MIT license. `HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1` successfully loaded it without network after installation. The sidecar health endpoint reported 384 dimensions. The probe verified finite, L2-normalized vectors and 32-item batches. Five batches reached 285.21 texts/s; single-query p50/p95/p99 was 9.90/12.53/239.98 ms, with an MPS tail outlier. Sidecar RSS was about 550 MB. Raw probe: `artifacts/neural_probe_offline.json`.

With `BEACON_EMBED_URL=http://127.0.0.1:8077`, `go test -run 'TestNeuralVsHashedQuality|TestNeuralExtendedQuality' -v ./internal/learning` exercised the actual HTTP embedder against 40 labeled paraphrase, synonym, hard-negative and exact-identifier cases. The added test corpus is in `neural_extended_test.go`; results are specific to these synthetic labels:

| Method | R@1 | R@5 | MRR |
|---|---:|---:|---:|
| Lexical | 3/40 = 0.075 | 3/40 = 0.075 | 0.075 |
| Hashed | 18/40 = 0.450 | 29/40 = 0.725 | 0.541 |
| BGE-small MPS | 26/40 = 0.650 | 36/40 = 0.900 | 0.757 |

Neural R@5 was 10/10 paraphrases, 6/10 synonyms, 10/10 hard negatives, and 10/10 exact identifiers. The previously published 17-case results were also reproduced on Mac for hashed (R@5 0.706) and neural (R@5 1.000); the lexical 0.412 value was from the earlier GPU-pod report. Error fallback, index versioning, and incremental update tests passed in the learning suite. Forty cases are too few for a general superiority claim; synonyms remain the weak category. The earlier RTX 3090 throughput and Mac blocker below are historical, not the current Mac measurements.

Reproduce after caching the model: start `embed_sidecar.py` in the local venv with offline environment variables, run `python3 reports/phase1/neural_probe.py`, and run the Go command above. No model cache or binaries are committed.

## Prior Phase 1 report (preserved below)

# Phase 1 — Neural Small-Scale

## Status

**CONDITIONAL PASS** — validated on the GPU pod (V6); the Mac was unreachable, so
Phase 1 could not re-run it on Apple Silicon.

## Mac blocker

`ssh <user>@192.0.2.10` →
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
