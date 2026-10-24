# Beacon Memory Engine V2 — adaptive approved-memory search

This document describes the indexed, cached, self-healing retrieval path added to
`internal/learning` for approved-memory search. Measured results are in
`SEARCH_BENCHMARKS.md`.

## Problem

`Store.ListMemories` originally answered a text query by selecting every
non-superseded row for the project (and kind), newest first, JSON-decoding every
row, and applying an in-Go matcher (`matchesText`: lowercase, split on
whitespace, AND of substring tests over ID, Title, Body, Kind).

When the scan terminates early (the requested rows are the newest), this is
already fast. When it does not — a rare term, a match that is old, a term that
matches nothing, or a high limit — the cost is proportional to the whole project,
including a JSON decode per row. That is the workload this change targets.

## Design

The authoritative data remains the `memories` table. Everything else is derived
and rebuildable.

```
Store.ListMemories
  ├─ no text        -> listMemoriesOrdered   (unchanged SQL path)
  └─ text query     -> searchMemories
                         ├─ short term / index unavailable -> listMemoriesScan
                         ├─ planner: selective?
                         │     no  -> listMemoriesScan
                         │     yes -> read generation
                         │            cache hit? -> return
                         │            fetchIndexedCandidates
                         │            exact filter (matchesText)
                         │            sort by updated_at DESC
                         │            paginate
                         │            cache put (under the read generation)
                         └─ on index error -> markIndexUnhealthy -> scan
```

### Derived index (`memories_fts`)

An FTS5 table with the trigram tokenizer and `case_sensitive 1` over the
lowercased haystack, the same tokenizer the endpoint history store already uses
(`internal/endpoint/dashboard/history_store.go`). The haystack is
`strings.ToLower(join([ID, Title, Body, Kind], "\n"))` — byte-identical to what
`matchesText` searched. A quoted trigram term matches exactly the rows containing
it as a substring, for terms of three or more characters. The index is
maintained in the same transaction as the authoritative row in `PutMemory`.

### Exactness

The indexed path only ever returns a superset of the original result set (for
terms of three or more runes), and every candidate is passed through the original
`matchesText` before ordering. A query with a term shorter than three runes
cannot be indexed by a trigram tokenizer and always uses the scan. The result is
therefore identical to the original scan; the differential suites hold it to
that.

### Planner

An index is not always faster. A common term's posting list is larger than the
whole scan it was meant to replace, and a scoped `MATCH` join over it is slower
than the scan (measured). The planner decides with a cheap, indexed statistic:

- `memory_trigram_df` holds the trigrams whose document frequency exceeds the
  candidate cap (256). It is a plain table with a primary-key index, rebuilt with
  the FTS index from `fts5vocab`. (An `fts5vocab` lookup directly is a full scan
  of the vocabulary — measured to be linear in corpus size — so it is used only
  to build this table, never per query.)
- A term is selective when at least one of its trigrams is absent from
  `memory_trigram_df`. The query is selective when any term is, since an AND is
  bounded by its most selective term.
- Each term is checked with one `IN (...)` statement, not one statement per
  trigram: a long common term like `workflow` has six trigrams, and six round
  trips were measurably slower than the scan they were meant to avoid.
- If the table is unavailable, the planner falls back to a bounded, join-free
  posting-list probe (`LIMIT cap+1`).

The generation is read only after the planner has chosen the indexed path, so a
common query that routes to the scan pays no reads beyond the original path.

### Result cache

A bounded (32 MiB), process-local LRU keyed by project, kind, query, limit,
page, and a ranking version. Correctness across processes rests on the durable
`generation` counter in `memory_index_meta`, advanced by `PutMemory` on every
write. The generation is read **before** the result is computed and the entry is
stored under that generation, so a write that commits during computation makes
the entry stale and it is never served. A cache hit costs one indexed single-row
read.

### Reliability

- **Fallback**: any error from the indexed path marks the index unhealthy and
  serves the original scan for that query. An index failure never returns an
  empty result as if it meant "no matches".
- **Write durability**: `PutMemory` never fails because of derived state. If the
  index row cannot be written, it marks the index not-ready and still commits the
  authoritative row and the generation advance.
- **Rebuild**: `index_ready` is cleared on any detected failure and on a detected
  missing index table; the next search rebuilds from `memories` inside one write
  transaction and flips `index_ready` back on. Rebuilds are serialized in-process
  and are idempotent.
- **Generation**: `PutMemory` updates the authoritative row, the index row, and
  the generation in one transaction, so no reader can observe a torn state.
- **Concurrency**: `_txlock=immediate` plus `_busy_timeout` make writes wait for
  the write lock instead of failing with `SQLITE_BUSY` on a lock upgrade. Schema
  and WAL setup happen once per process per path, and index readiness is cached
  in-process, so reads take no write lock and avoid repeated flag reads.

## Invariants

1. `memories` is authoritative; the index, cache and vocabulary are derived and
   can be dropped and rebuilt.
2. An indexed result equals the scan result for the same query.
3. Project isolation is enforced in every path (index join and scan).
4. A write is visible to every process on its next query (durable generation),
   and a write never fails because of derived-index state.
5. A derived-index failure degrades to the scan; it never changes the answer.

## Modes

`ListMemories` keeps its existing signature and behaviour, so it is the exact
compatibility mode. The adaptive path is selected automatically and is exact, so
no caller changes. A separately scored semantic mode is not implemented.

## Not implemented

Semantic retrieval / evidence-aware ranking, evidence trust metadata and
dependency invalidation, and ANN are not implemented. They require an embedding
backend and a labelled relevance set that are out of scope for this change. No
stubs are present that could be mistaken for working code.
