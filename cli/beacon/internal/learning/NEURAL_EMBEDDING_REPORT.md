# Neural embedding report

Status: **NOT IMPLEMENTED** (backend not integrated). Interface and storage exist
from V3/V4.

## Models and runtimes evaluated

| option | runtime | license | result |
|---|---|---|---|
| BAAI/bge-small-en-v1.5 | ONNX Runtime | MIT (model) | **not integrated** |
| sentence-transformers/all-MiniLM-L6-v2 | ONNX Runtime / PyTorch | Apache-2.0 | **not integrated** |
| fastembed-go | ONNX Runtime | Apache-2.0 | **archived** — rejected on maintenance risk |
| onnxruntime_go | native ONNX Runtime shared lib (cgo) | MIT | requires a per-platform native shared library; not present and not installable here without cgo + a redistributable binary |
| sqlite-vec / sqlite-vector | C extension | permissive | a vector store, not an embedder; also needs a loadable extension the pure-Go driver does not load |

## Why nothing was selected

All neural paths require at least one of: a native shared library (cgo), a model
file download, or a hosted service. In this environment none could be installed
and validated offline, and bundling an unvalidated native binary would violate
the "no silent third-party binary" rule. Reporting neural numbers without
running a model would be fabrication, so none are reported.

## What exists and is ready

- `Embedder` interface (`Model`, `Dim`, `Embed`) — a neural backend drops in
  behind it without touching storage or fusion.
- `memory_embeddings(memory_id, project_id, model, content_hash, dim, vector)`
  stores vectors keyed by model + content hash, so a model change invalidates
  only that model's vectors. This is exactly the model-version-aware invalidation
  a neural backend needs.
- Incremental updates (`upsertEmbeddingTx`) and batch backfill
  (`RebuildEmbeddings`) already run per-memory; no full reindex on write.

## Requirement checklist (against the brief)

| requirement | status |
|---|---|
| pluggable backend | yes (interface) |
| local CPU execution | not demonstrated (no runtime) |
| optional init | yes (opt-in via `OpenSemantic`) |
| explicit offline install | not implemented (no model) |
| versioned metadata | yes (model column + content hash) |
| incremental updates | yes |
| model-version invalidation | yes (per-model rows) |
| batch embedding | yes (`RebuildEmbeddings`) |
| safe concurrency | not validated for a neural backend |
| graceful fallback | yes (no embedder → lexical only) |
| no mixed models in one index | yes (queries filter by `model`) |
| no silent download | yes (no download code exists) |
| checksum/license | not implemented (no model shipped) |

## Recommendation

To land neural retrieval, vendor a license-cleared ONNX model plus a pure-Go or
per-platform-vendored ONNX runtime, add a checksum + explicit install step, and
gate it behind `OpenSemantic`. Until then, the hashed embedder remains the only
working semantic backend and its limits (no true synonymy) stand.
