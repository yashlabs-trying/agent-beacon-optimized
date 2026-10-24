# Changelog

All notable changes to the **memory-engine optimization and hardening work** in this
lineage are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

> **Scope.** This changelog covers the optimization/hardening commits layered on top of
> upstream Agent Beacon V7 development (`7b605aa84`). It is not the upstream product
> changelog; see the upstream project for product releases. Numbers are reproducible
> metrics from `reports/`; unmeasured or blocked items are labeled.

## [Unreleased]

### Added
- Safe local database placement: `BEACON_MEMORY_DIR`, mount detection
  (`CheckPlacement`), and transactional migration (`MigrateDatabase`, `VACUUM INTO`).
  (`2bcc2560`)
- Opt-in authenticated memory provenance: Ed25519, content-bound attestations with a
  `TrustRegistry`, fail-safe verification. (`500e2a0e`)
- Project authorization for memory reads/search/context (fail-closed allowlist).
  (`2d` series: `2f148293`, `227ce043`, `ad9b0256`)
- Protected Ed25519 approval signing (`ff2a0a5c`), append-only security audit log
  (`bedd73b1`), context-aware deadlines/cancellation (`545cf3b9`), Windows DPAPI key
  protection (`1f3122d9`).
- `examples/memory-basic`, `examples/memory-scale`, `examples/agent-mcp`, and the
  `docs/optimization/` documentation set (this change).

### Changed
- **Write concurrency:** process-level `writeMu` serializes in-process writers;
  16-client write p99 **634 ms → 48–54 ms** on local ext4, durability unchanged.
  (`d32e9d3b`, `0b6dbe05`, `004f780d`)
- **Retrieval allocations:** question-aware selection computes term sets once and
  shares them; **2,007 → 875 allocs/op (−56.4%)**, bytes −37.3%. (`90b21107`)
- Batched trigram selectivity check for long multi-term queries. (`92e910bf`)
- High-performance transactional batch writes. (`7cbef131`)

### Fixed
- Question-attribute resolution now uses the first content noun, not the last. (`e2408766`)
- Supersession regression and an approval/supersession race. (`ff2b1447`, `df968893`)
- S4 lifecycle trust boundary pinned by test. (`db341fbc`)
- MCP server now honors `BEACON_MEMORY_DIR`. (`37419b62`)

### Security
- Hardened the local threat model: cross-project read enforcement (allowlist mode),
  atomic lifecycle, bounded search, request-shape limits, shared store.
  See `reports/security/`.

### Known limitations
- Authenticated-provenance **deployment** is BLOCKED (no protected key authority ships).
- Scale (64 clients, 1M/10M memories), full CLI suite, MCP/Cross-platform integration,
  and a fresh semantic-vs-lexical ablation are **NOT TESTED**.
- No agent-intelligence improvement is claimed. See `docs/optimization/limitations.md`.

## Provenance

Frozen comparators: Original `99c70908`, V6 `209a987b`, Phase 5 `02e34f98`,
V7-dev `7b605aa8`. Optimization HEAD: `db341fbc`. The full commit map is in
`docs/optimization/roadmap.md`.
