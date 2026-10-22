#!/usr/bin/env python3
"""Minimal local embedding sidecar for Beacon.

Loads a sentence-transformers model and serves:
  GET  /health          -> {"model":..., "dim":..., "device":...}
  POST /embed {"texts":[...]} -> {"model":..., "dim":..., "vectors":[[...]]}

Local-only, offline after the model is cached. No external calls.
"""
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MODEL_NAME = os.environ.get("BEACON_EMBED_MODEL", "BAAI/bge-small-en-v1.5")
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
    print(f"serving {MODEL_NAME} dim={DIM} device={device} on 127.0.0.1:{PORT}", flush=True)
    ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
