#!/usr/bin/env python3
"""Phase 4A host-binding variant of the Beacon embedding sidecar.

Identical contract to cli/beacon/internal/learning/embed_sidecar.py, with one
change: the listen address is configurable via BEACON_EMBED_HOST so the Mac M5
sidecar can serve a cross-host retrieval run (WSL2 client). The model contract,
endpoints, and normalization are unchanged so results stay comparable to the
in-repo sidecar.

  GET  /health               -> {"model":..., "dim":..., "device":...}
  POST /embed {"texts":[...]} -> {"model":..., "dim":..., "vectors":[[...]]}

Local-only; no external calls.
"""
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MODEL_NAME = os.environ.get("BEACON_EMBED_MODEL", "BAAI/bge-small-en-v1.5")
HOST = os.environ.get("BEACON_EMBED_HOST", "127.0.0.1")
PORT = int(os.environ.get("BEACON_EMBED_PORT", "8077"))

import torch  # noqa: E402
from sentence_transformers import SentenceTransformer  # noqa: E402

if torch.cuda.is_available():
    device = "cuda"
elif torch.backends.mps.is_available():
    device = "mps"
else:
    device = "cpu"
model = SentenceTransformer(MODEL_NAME, device=device)
DIM = model.get_sentence_embedding_dimension()


class Handler(BaseHTTPRequestHandler):
    def _send(self, code, obj):
        body = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        self._send(200, {"model": MODEL_NAME, "dim": DIM, "device": device})

    def do_POST(self):
        if self.path != "/embed":
            self._send(404, {"error": "not found"})
            return
        n = int(self.headers.get("Content-Length", "0"))
        try:
            payload = json.loads(self.rfile.read(n) or b"{}")
        except Exception as exc:  # noqa: BLE001
            self._send(400, {"error": str(exc)})
            return
        texts = payload.get("texts", [])
        vecs = model.encode(
            texts, normalize_embeddings=True, batch_size=64, convert_to_numpy=True
        ).tolist()
        self._send(200, {"model": MODEL_NAME, "dim": DIM, "vectors": vecs})

    def log_message(self, *args):  # silence default logging
        pass


if __name__ == "__main__":
    print(f"serving {MODEL_NAME} dim={DIM} device={device} on {HOST}:{PORT}", flush=True)
    ThreadingHTTPServer((HOST, PORT), Handler).serve_forever()
