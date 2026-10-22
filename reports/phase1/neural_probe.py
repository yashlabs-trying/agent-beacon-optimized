"""Probe the real local embedding sidecar; run after model cache is available."""

import json
import math
import statistics
import sys
import time
import urllib.request


def request(url, payload=None):
    data = None if payload is None else json.dumps(payload).encode()
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=30) as response:
        return json.load(response)


def main():
    base = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8078"
    health = request(base + "/health")
    texts = [f"Recover the memory index after a failed write in project {i}." for i in range(32)]
    batch_times = []
    for _ in range(5):
        started = time.perf_counter()
        output = request(base + "/embed", {"texts": texts})
        batch_times.append(time.perf_counter() - started)
        vectors = output["vectors"]
        assert len(vectors) == 32
        assert all(len(v) == health["dim"] for v in vectors)
        assert all(math.isfinite(x) for v in vectors for x in v)
        assert all(abs(sum(x * x for x in v) - 1) < 1e-3 for v in vectors)
    single_times = []
    for text in texts[:30]:
        started = time.perf_counter()
        output = request(base + "/embed", {"texts": [text]})
        single_times.append(time.perf_counter() - started)
        assert len(output["vectors"]) == 1
    single_times.sort()
    result = {
        "health": health,
        "batch_size": len(texts),
        "batches": len(batch_times),
        "batch_throughput_texts_per_second": round(32 * 5 / sum(batch_times), 2),
        "single_query_p50_ms": round(statistics.median(single_times) * 1000, 2),
        "single_query_p95_ms": round(single_times[28] * 1000, 2),
        "single_query_p99_ms": round(single_times[29] * 1000, 2),
        "normalized": True,
        "finite": True,
    }
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
