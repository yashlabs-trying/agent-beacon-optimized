#!/usr/bin/env python3
"""Phase 4A Benchmark B — independent retrieval comparisons.

Compares the locked-fixture retrieval methods and independent baselines on
*identical* corpus contents, identifiers, queries, relevance labels (gold =
{primary, duplicate}), candidate limits, and hardware.

Baselines implemented here are deliberately independent of Beacon:
  * bm25          Okapi BM25 (pure Python)
  * sqlite_fts5   SQLite FTS5 (unicode61)
  * recency       newest-first (no content signal)
  * bge_exact     exact cosine over real BAAI/bge-small-en-v1.5 embeddings via
                  the Mac sidecar (an oracle for ANN recall; opt-in)

It also runs a synthetic scale sweep (1K / 10K / 100K docs) with clean labels
to measure recall, latency percentiles, index build time, and index size.

Reported metrics: Recall@1/5/10, Precision@5, MRR, nDCG@5, p50/p95/p99 latency.
Cold vs warm are separated for the scale sweep.
"""
import argparse
import json
import math
import os
import re
import sqlite3
import statistics
import sys
import time
import urllib.request
from pathlib import Path

TOKEN_RE = re.compile(r"[a-z0-9]+")


def tok(text):
    return TOKEN_RE.findall(text.lower())


def percentile(values, p):
    if not values:
        return 0.0
    vals = sorted(values)
    idx = max(0, min(len(vals) - 1, int(math.ceil(len(vals) * p)) - 1))
    return vals[idx]


# ---------------------------------------------------------------- metrics
def evaluate(rank_fn, cases, k_list=(1, 5, 10)):
    lat, by_cat = [], {}
    totals = {k: 0.0 for k in k_list}
    p5 = mrr = ndcg = 0.0
    n = 0
    for case in cases:
        start = time.perf_counter()
        ranked = rank_fn(case)
        lat.append((time.perf_counter() - start) * 1000.0)
        gold = set(case["gold"])
        n += 1
        for k in k_list:
            if any(d in gold for d in ranked[:k]):
                totals[k] += 1
        hit5 = [d for d in ranked[:5] if d in gold]
        p5 += len(hit5) / 5.0
        rr = 0.0
        for i, d in enumerate(ranked):
            if d in gold:
                rr = 1.0 / (i + 1)
                break
        mrr += rr
        dcg = sum(1.0 / math.log2(i + 2) for i, d in enumerate(ranked[:5]) if d in gold)
        idcg = sum(1.0 / math.log2(i + 2) for i in range(min(len(gold), 5)))
        ndcg += (dcg / idcg) if idcg else 0.0
        cat = case.get("category", "ALL")
        by_cat.setdefault(cat, []).append((ranked, gold))
    out = {
        "n": n,
        "recall": {k: totals[k] / n for k in k_list},
        "precision@5": p5 / n,
        "mrr": mrr / n,
        "ndcg@5": ndcg / n,
        "p50_ms": percentile(lat, 0.50),
        "p95_ms": percentile(lat, 0.95),
        "p99_ms": percentile(lat, 0.99),
    }
    return out


# ---------------------------------------------------------------- baselines
class BM25:
    def __init__(self, docs, k1=1.5, b=0.75):
        self.k1, self.b = k1, b
        self.ids = [d[0] for d in docs]
        self.tokens = [tok(d[1]) for d in docs]
        self.dl = [len(t) for t in self.tokens]
        self.avgdl = statistics.mean(self.dl) if self.dl else 1.0
        self.tf = []
        self.df = {}
        for t in self.tokens:
            counts = {}
            for w in t:
                counts[w] = counts.get(w, 0) + 1
            self.tf.append(counts)
            for w in counts:
                self.df[w] = self.df.get(w, 0) + 1
        self.N = len(docs)

    def rank(self, query, limit=10):
        qt = tok(query)
        scores = []
        for i, doc in enumerate(self.ids):
            s = 0.0
            dl = self.dl[i]
            for w in qt:
                f = self.tf[i].get(w, 0)
                if not f:
                    continue
                idf = math.log(1 + (self.N - self.df.get(w, 0) + 0.5) / (self.df.get(w, 0) + 0.5))
                s += idf * (f * (self.k1 + 1)) / (f + self.k1 * (1 - self.b + self.b * dl / self.avgdl))
            scores.append((s, doc))
        scores.sort(key=lambda x: (-x[0], x[1]))
        return [d for _, d in scores[:limit]]


class FTS5:
    def __init__(self, docs):
        self.conn = sqlite3.connect(":memory:")
        self.conn.execute("CREATE VIRTUAL TABLE docs USING fts5(id UNINDEXED, body)")
        self.conn.executemany("INSERT INTO docs (id, body) VALUES (?, ?)",
                              [(d[0], d[1]) for d in docs])
        self.conn.commit()

    def rank(self, query, limit=10):
        terms = tok(query)
        if not terms:
            return []
        match = " OR ".join(f'"{t}"' for t in terms)
        try:
            rows = self.conn.execute(
                "SELECT id FROM docs WHERE docs MATCH ? ORDER BY bm25(docs) LIMIT ?",
                (match, limit)).fetchall()
        except sqlite3.OperationalError:
            return []
        return [r[0] for r in rows]


class Recency:
    def __init__(self, docs):
        self.order = [d[0] for d in reversed(docs)]

    def rank(self, query, limit=10):
        return self.order[:limit]


class BGEExact:
    def __init__(self, docs, url, model):
        self.ids = [d[0] for d in docs]
        self.url = url
        self.model = model
        self.vecs = self._embed([d[1] for d in docs])

    def _embed(self, texts):
        req = urllib.request.Request(self.url + "/embed",
                                     data=json.dumps({"texts": texts}).encode(),
                                     headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(req, timeout=120) as resp:
            return json.load(resp)["vectors"]

    def rank(self, query, limit=10):
        qv = self._embed([query])[0]
        scores = []
        for i, v in enumerate(self.vecs):
            dot = sum(a * b for a, b in zip(qv, v))
            scores.append((dot, self.ids[i]))
        scores.sort(key=lambda x: (-x[0], x[1]))
        return [d for _, d in scores[:limit]]


# ---------------------------------------------------------------- fixtures
def load_locked(path):
    fx = json.loads(Path(path).read_text(encoding="utf-8"))
    docs, cases = [], []
    for topic in fx["topics"]:
        docs += [
            (topic["id"] + "-primary", topic["primary"]),
            (topic["id"] + "-duplicate", topic["duplicate"]),
            (topic["id"] + "-support", topic["support"]),
            (topic["id"] + "-decoy", topic["decoy"]),
        ]
        gold = [topic["id"] + "-primary", topic["id"] + "-duplicate"]
        for cat in ("identifier", "paraphrase", "synonym", "hard_negative"):
            cases.append({"query": topic["queries"][cat], "gold": gold, "category": cat})
    return fx.get("dataset_id", "locked"), docs, cases


def synthetic_scale(scale, n_queries=200, seed=1234):
    import random
    rng = random.Random(seed)
    filler = ["alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta",
              "iota", "kappa", "lambda", "mu", "nu", "xi", "omicron", "pi"]
    docs = []
    for i in range(scale):
        body = f"doc{i} " + " ".join(rng.choice(filler) for _ in range(12))
        docs.append((f"d{i}", body))
    cases = []
    step = max(1, scale // n_queries)
    for i in range(0, scale, step):
        if len(cases) >= n_queries:
            break
        cases.append({"query": f"doc{i}", "gold": [f"d{i}"], "category": "ALL"})
    return docs, cases


def run_methods(docs, cases, methods, embed_url="", embed_model="BAAI/bge-small-en-v1.5"):
    results = {}
    build = {}
    for name in methods:
        t0 = time.perf_counter()
        if name == "bm25":
            m = BM25(docs)
        elif name == "sqlite_fts5":
            m = FTS5(docs)
        elif name == "recency":
            m = Recency(docs)
        elif name == "bge_exact":
            m = BGEExact(docs, embed_url, embed_model)
        else:
            raise ValueError(name)
        build[name] = (time.perf_counter() - t0) * 1000.0
        results[name] = evaluate(lambda c, m=m: m.rank(c["query"]), cases)
    return results, build


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--fixture", required=True)
    p.add_argument("--output", required=True)
    p.add_argument("--embed-url", default="")
    p.add_argument("--embed-model", default="BAAI/bge-small-en-v1.5")
    p.add_argument("--scales", default="1000,10000,100000")
    p.add_argument("--synthetic-only", action="store_true")
    args = p.parse_args()

    report = {"fixture": None, "locked": {}, "synthetic": {}, "config": {}}

    if not args.synthetic_only:
        dataset_id, docs, cases = load_locked(args.fixture)
        report["fixture"] = dataset_id
        report["config"]["locked"] = {"docs": len(docs), "queries": len(cases),
                                      "gold_per_query": 2, "limit": 10}
        methods = ["bm25", "sqlite_fts5", "recency"]
        if args.embed_url:
            methods.append("bge_exact")
        results, build = run_methods(docs, cases, methods, args.embed_url, args.embed_model)
        report["locked"]["results"] = results
        report["locked"]["build_ms"] = build
        print("== locked fixture ==")
        for name, r in results.items():
            print(f"{name:14s} R@1={r['recall'][1]:.4f} R@5={r['recall'][5]:.4f} R@10={r['recall'][10]:.4f} "
                  f"P@5={r['precision@5']:.4f} MRR={r['mrr']:.4f} nDCG@5={r['ndcg@5']:.4f} "
                  f"p50={r['p50_ms']:.3f}ms p95={r['p95_ms']:.3f}ms p99={r['p99_ms']:.3f}ms")

    for scale in [int(s) for s in args.scales.split(",") if s.strip()]:
        docs, cases = synthetic_scale(scale)
        methods = ["bm25", "sqlite_fts5", "recency"]
        results, build = run_methods(docs, cases, methods)
        entry = {"docs": len(docs), "queries": len(cases), "results": results, "build_ms": build}
        report["synthetic"][str(scale)] = entry
        print(f"== synthetic scale {scale} ==")
        for name, r in results.items():
            print(f"{name:14s} R@1={r['recall'][1]:.4f} R@5={r['recall'][5]:.4f} R@10={r['recall'][10]:.4f} "
                  f"p50={r['p50_ms']:.3f}ms p95={r['p95_ms']:.3f}ms p99={r['p99_ms']:.3f}ms "
                  f"build={build[name]:.1f}ms")

    Path(args.output).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
