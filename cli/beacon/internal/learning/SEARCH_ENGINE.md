# Beacon Memory Engine V2 — adaptive approved-memory search

This document describes the indexed, cached, self-healing retrieval path added to
`internal/learning` for approved-memory search. It is the design record for the
change; the measured results are in `SEARCH_BENCHMARKS.md`.

## Problem

`Store.ListMemories` originally answered a text query by:

1. selecting every non-superseded row for the project (and kind), newest first,
2. JSON-decoding every row, and
3. applying an in-Go matcher (`matchesText`: lowercase, split on whitespace, AND
   of substring tests over ID, Title, Body, Kind).

When the scan terminates early (the requested rows are the newest), this is
already fast. When it does not — a rare term, a match that is old, a term that
matches nothing, or a high limit — the cost is proportional to the whole
project, including a JSON decode per row. That is the workload this change
targets.

## Design

The authoritative data remains the `memories` table. Everything else is derived
and rebuildable.

```
Store.ListMemories
  ├─ no text      -> listMemoriesOrdered   (unchanged SQL path)
  └─ text query   -> searchMemories
                       ├─ result cache hit? -> return
                       ├─ planner: selective? -> fetchIndexedCandidates
                       │                          ├─ exact filter (matchesText)
                       │                          ├─ sort by updated_at DESC
                       │                          └─ paginate
                       └─ otherwise / on error -> listMemoriesScan (original)
```

### Derived index (`memories_fts`)

An FTS5 table with the trigram tokenizer and `case_sensitive 1` over the
lowercased haystack, the same tokenizer the endpoint history store already uses
(`internal/endpoint/dashboard/history_store.go`). The haystack is
`strings.ToLower(join([ID, Title, Body, Kind], "\n"))` — byte-identical to what
`matchesText` searched. A quoted trigram term matches exactly the rows
containing it as a substring, for terms of three or more characters. The index
is maintained in the same transaction as the authoritative row in `PutMemory`.

### Exactness

The indexed path only ever returns a superset of the original result set (for
terms of three or more runes), and every candidate is passed through the
original `matchesText` before ordering. A query with a term shorter than three
runes cannot be indexed by a trigram tokenizer and always uses the scan. The
result is therefore identical to the original scan; the 23-case comparison suite
and the 10,000-case randomized check hold it to that.

### Planner

An index is not always faster. A common term's posting list is larger than the
whole scan it was meant to replace, and a scoped `MATCH` join over it is slower
than the scan (measured). The planner decides with a cheap, indexed statistic:

- `memory_trigram_df` holds the trigrams whose document frequency exceeds the
  candidate cap (256). It is a plain table with a primary-key index, rebuilt with
  the FTS index from `fts5vocab`. (An `fts5vocab` lookup directly is a full scan
  of the vocabulary — measured to be linear in corpus size — so it is only used
  to build this table, never per query.)
- A term is selective when at least one of its trigrams is absent from
  `memory_trigram_df`. The query is selective when any term is, since an AND is
  bounded by its most selective term.
- If the table is unavailable, the planner falls back to a bounded, join-free
  posting-list probe (`LIMIT cap+1`).

### Result cache

A bounded (32 MiB), process-local LRU keyed by project, kind, query, limit,
page, and a ranking version. Correctness across processes rests on the durable
`generation` counter in `memory_index_meta`, advanced by `PutMemory` on every
write. A cache entry is served only when the generation read from the database
still matches; the check is one indexed single-row read. The cache never holds a
result produced by an older generation.

### Reliability

- **Fallback**: any error from the indexed path marks the index unhealthy and
  serves the original scan for that query. An index failure never returns an
  empty result as if it meant "no matches".
- **Rebuild**: `index_ready` is cleared on any detected failure and on a
  detected missing index table; the next search rebuilds from `memories` inside
  one write transaction and flips `index_ready` back on. Rebuilds are serialized
  in-process and are idempotent.
- **Generation**: `PutMemory` updates the authoritative row, the index row, and
  the generation in one transaction, so no reader can observe a torn state.
- **Concurrency**: `_txlock=immediate` plus `_busy_timeout` make writes wait for
  the write lock instead of failing with `SQLITE_BUSY` on a lock upgrade. Schema
  and WAL setup happen once per process per path, so reads take no write lock.

## Invariants

1. `memories` is authoritative; the index, cache, and vocabulary are derived and
   can be dropped and rebuilt.
2. An indexed result equals the scan result for the same query (proven by the
   comparison suite).
3. Project isolation is enforced in every path (index join and scan).
4. A write is visible to every process on its next query (durable generation).
5. A derived-index failure degrades to the scan; it never changes the answer.

## Modes

`ListMemories` keeps its existing signature and behaviour, so it is the exact
compatibility mode. The adaptive path is selected automatically and is exact, so
no caller changes. A future, separately scored semantic mode is not implemented;
see the limitations in `SEARCH_BENCHMARKS.md`.

## Not implemented

Components 2 (semantic retrieval / evidence-aware ranking), 3 (evidence trust
metadata and dependency invalidation), and the semantic parts of 5 are not
implemented. They require an embedding backend and a labelled relevance set that
are out of scope for this change. The trust and semantic hooks are not stubbed in
a way that could be mistaken for working code.
