# Optimization case study

Engineering documentation for the **memory-engine optimization and hardening work**
layered on top of upstream [Agent Beacon](https://github.com/Asymptote-Labs/agent-beacon).

> **Attribution.** Beacon is an open-source project by Asymptote Labs (MIT,
> `Copyright (c) 2026 Asymptote Labs`). This repository is that project plus a series of
> optimization/hardening commits. The upstream product, documentation, and license
> notices are preserved. See [`CONTRIBUTING.md`](../../CONTRIBUTING.md) and
> [Repository scope](#repository-scope).

## Start here

| If you are… | Read |
|---|---|
| New to this work | the [architecture](architecture.md) for the data flow and the three optimization themes |
| Here for the numbers | [performance](performance.md) |
| Trying to reproduce | [benchmarks](benchmarks.md) |
| Reviewing security | [security](security.md) |
| Sizing up what's real | [limitations](limitations.md) — read this before quoting anything |
| Planning next work | [roadmap](roadmap.md) |
| Setting up | [configuration](configuration.md) |

## The 60-second version

Beacon is a **local-first memory layer** for AI coding agents: it captures agent session
history across many harnesses and serves reviewed project memory to agents over MCP.
The optimization work changes three things in the memory engine and leaves the product
interfaces and defaults intact:

1. **Write concurrency** — a process-level mutex serializes in-process writers instead
   of letting 16 connections busy-poll SQLite's write lock. 16-client write p99 drops
   **634 ms → 48–54 ms** with durability unchanged.
2. **Retrieval allocations** — term sets are computed once and shared. Question-aware
   retrieval drops **56.4% of allocations** (2,007 → 875 allocs/op).
3. **Placement + lifecycle** — a configurable local database directory with mount
   detection and safe migration, plus authorization, atomic lifecycle, resource bounds,
   and an opt-in Ed25519 provenance mechanism.

**What this does not claim:** better retrieval quality, smarter agents, or production
readiness. Those are unmeasured or blocked — see [limitations](limitations.md).

## Repository scope

This repository contains the upstream Beacon product **and** the optimization evidence.
Concretely:

- **Product code + upstream docs:** `cli/`, `pkg/`, `plugins/`, `docs/` (Mintlify site),
  `packaging/`, `rules/`, `spec/`, `beacon-sandbox/`, `examples/`.
- **Optimization evidence:** `reports/` (v7, v7-hardening, security, phase trees), and
  this `docs/optimization/` section.
- **Optimization commits:** see the [commit map](roadmap.md#commit-map).

Our contribution is **additive**: it does not rename public APIs, remove benchmarks, or
change established default behavior. Upstream attribution and the MIT license are
unchanged.

## Verify on this machine

```sh
cd cli/beacon
go build ./...
go test ./internal/learning/ -count=1
```

Then reproduce the headline result in [benchmarks](benchmarks.md#1-write-concurrency-the-headline-result).
