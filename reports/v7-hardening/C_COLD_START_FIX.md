# Beacon V7 hardening — Phase 2: Cold-Start Fix

Status: **PARTIAL — root cause identified and bounded. Healthy-store cold retrieval now
matches original; the only remaining cost is the necessary one-time index build.**

## 1. Reproduction of the reported penalty

Phase A `cold_cli_summary_v2.txt` (56 tasks, new process per call):

| Pass | original mean | V7 mean |
|---|---:|---:|
| first | 14.66 ms | 43.71 ms |
| repeat | 13.81 ms | 14.85 ms |

- V7 **first** call: 43.7 ms (55/56 slower than original).
- V7 **repeat** call: 14.85 ms — **parity** with original (13.81 ms).
- The penalty is identical for the lexical route (42.3 ms) and the semantic route
  (47.2 ms), so it is **not** semantic/embedding work.

## 2. Root cause

The derived lexical index (`memories_fts`, a **trigram** FTS5 table) is built **lazily on
the first query** when `memory_index_meta.index_ready = 0`. Seeding writes memories
through `PutMemory`, which deliberately does **not** maintain the derived index while
`index_ready = 0`. So after a fresh seed the index is empty, and the first query runs
`rebuildMemoryIndexTx` — a full trigram-index build over the store. Every subsequent
process finds `index_ready = 1` and pays nothing.

This is the "first-use FTS readiness" the Phase A audit named, now pinned to the exact
mechanism.

## 3. Verified behaviour (this host, WSL2)

Seeded store (one Phase 5 task), then new-process CLI calls:

| Call | ms |
|---|---:|
| V7 first call (builds index) | 26 |
| original cold ×3 | 65, 20, 23 |
| V7 cold ×3 **after index built** | 20, 36, 20 |

Once the index exists, V7 cold retrieval is at **parity** with original. A healthy store
does **not** rebuild its index on a new process — `ensureIndexReady` short-circuits on
`index_ready = 1`.

## 4. What was tried and reverted

A `user_version` fast-path in `ensureSchema` that skips the 9 `CREATE ... IF NOT EXISTS`
statements when the schema version already matches. Measured effect on
`BenchmarkPhaseCOpenColdExisting` (open + one query on an existing 1,000-memory store):
baseline 493–610 µs vs 526–795 µs after — **no measurable benefit** (the cost is the
trigram query, not the DDL). Per the discipline "no measured benefit → revert", the
change was **reverted**.

## 5. Acceptance

| Requirement | Status |
|---|---|
| Healthy existing database does not rebuild on new process | **PASS** (index_ready=1 short-circuit; repeat parity) |
| Cold retrieval on a healthy store matches/beats original | **PASS** (20–36 ms vs 20–65 ms) |
| No stale/missing results | **PASS** (index rebuild only when absent; correctness tests green) |
| New-installation first request | **PARTIAL** — pays the one-time trigram build (necessary) |
| Real initialization cost reported | **PASS** (26 ms build on this fixture) |

## 6. Remaining work (not done)

To remove the first-build cost from the read path, the index would need to be built
**during ingest** (incremental maintenance while `index_ready = 0`) or during an explicit
initialization step. That is a write-path change with correctness implications (partial
index must never be trusted) and was **not implemented** here; the current lazy build is
correct and one-time. A cheaper tokenizer (word-level vs trigram) is a larger change and
is out of scope.

## 7. Verdict

**PARTIAL.** The unnecessary-rebuild concern is resolved (healthy stores never rebuild).
The measured first-request penalty is a necessary one-time index build, after which V7 is
at parity with original. Target "cold retrieval ≤ original on healthy stores": **PASS**.
