# Phase 4A — Benchmark C: Local Model Readiness

**Gate: PASS — a protocol-compatible local model exists and passes the capability check.**

- Raw evidence: `RAW_EVIDENCE/model_capability_qwen2.5-coder-7b.json`,
  `RAW_EVIDENCE/model_capability_qwen2.5-coder-1.5b.json`
- Check driver: `reports/phase4/part1/harness/model_capability_check.py`

## 1. Host inventory

| Host | Reachable | RAM | Free disk | Runtimes | Cached weights |
|---|---|---|---|---|---|
| WSL2 laptop (`lappiiii`) | local | 7.6 GiB | ~332 GiB | Go 1.27.3, Python 3.14.4, torch 2.14.0+cpu, transformers 5.17.0 | none for this task |
| Apple M5 Mac (`192.0.2.10`) | **yes (SSH + LAN)** | 16 GiB | 332 GiB | Python 3.12 venv: torch 2.14.1, sentence-transformers 6.2.0; Ollama 0.40.6; MPS GPU | **BGE-small (133 MB), qwen2.5-coder:1.5b, qwen2.5-coder:7b** |

The Mac was `192.0.2.4` and unreachable during Phases 3A–3D; it is now on
`192.0.2.10` and reachable. This is what unblocked both Phase 3D gates.

## 2. What changed versus Phase 3D

| Prerequisite | Phase 3D | Phase 4A |
|---|---|---|
| Apple M5 reachable | BLOCKED | **YES** |
| `BAAI/bge-small-en-v1.5` weights | absent | **cached (133 MB, MPS)** |
| `sentence-transformers` | missing | **present (venv 3.12)** |
| Local coder model | none usable | **qwen2.5-coder:7b + 1.5b (Ollama)** |

No model was downloaded in Phase 4A; all weights were already cached.

## 3. Tool-calling capability check

The check drives the model through the same tool schema as the six-task harness against a
tiny sandboxed project (`inc.py` must become `x+1`) and verifies: syntax, argument
validity, tool-result consumption, project modification, and final output.

### qwen2.5-coder:7b — SUITABLE

```json
{
  "tool_call_syntax_ok": true,
  "tool_arguments_valid": true,
  "consumed_tool_results": true,
  "used_write_file": true,
  "used_run_tests": true,
  "fixed_project_tests_pass": true,
  "final_output_present": true,
  "call_sources": ["text_json"],
  "final_text": "DONE",
  "suitable": true
}
```

### qwen2.5-coder:1.5b — UNSUITABLE

```json
{
  "used_write_file": true,
  "used_run_tests": false,
  "fixed_project_tests_pass": false,
  "suitable": false
}
```

The 1.5b model repeatedly rewrote the same wrong body and never ran the tests; it is
**classified unsuitable** and is *not* used for the agent benchmark. Any downstream task
failure from this model must not be read as a memory-system failure.

## 4. Model invocation facts

- Endpoint: `http://192.0.2.10:11434/api/chat` (Ollama `OLLAMA_HOST=0.0.0.0`).
- Decoding: `temperature=0, seed=42, num_ctx=4096, num_predict=512` (harness defaults).
- Device: Apple M5 Metal GPU (Ollama log: `library=Metal … Apple M5`).
- Tool-call transport: **JSON-in-content** (Ollama did not return native `tool_calls` for
  this model); the harness fallback parses it (see `HARNESS_VALIDATION.md`).

## 5. Neural runtime facts

- Sidecar: `reports/phase4/part1/harness/embed_sidecar_host.py`, model
  `BAAI/bge-small-en-v1.5`, dim 384, device `mps`, serving on `0.0.0.0:8077`.
- `/health` → `{"model":"BAAI/bge-small-en-v1.5","dim":384,"device":"mps"}`.
- Loaded offline (`HF_HUB_OFFLINE=1`), from the local cache only.

## Conclusion

A compatible local coder model (qwen2.5-coder:7b) and a real neural embedder (BGE-small)
are both available and verified. **Benchmark C: PASS.**
