# Limitations

This page is deliberately blunt. It exists so that no reader over-claims what the
optimization work establishes.

## What is established (reproducible)

- The **write-concurrency tail** is fixed on a local filesystem: 16-client write p99
  **634 ms → 48–54 ms**, zero errors/busy, durability unchanged.
- **Question-aware retrieval allocations** are down **56.4%** (2,007 → 875 allocs/op),
  bytes down 37.3%, with no correctness/trust/cache regression.
- **Cold healthy-store retrieval** is at parity with the original (20–36 ms vs
  20–65 ms); a first-ever query pays a one-time trigram index build.
- **Database placement** is configurable, synchronized mounts are detected and warned
  about, and a transactional migration exists.
- **Authorization, atomic lifecycle, resource bounds, and an opt-in Ed25519 provenance
  mechanism** are implemented with passing adversarial tests.
- Core suite, race subset, `go vet`, and `go build ./...` pass on the reference host.

## What is NOT established

- **No agent-intelligence improvement.** Nothing here shows the agent becomes more
  accurate or more successful. The one model-backed dataset (56-task pilot) is
  **historical**, covers **7 repeated mechanism families**, and is not independent
  evidence. A family-level sign test gives p=0.5.
- **No speed advantage on the retrieval wall-clock.** The warm path is SQLite/I/O
  bound; wall-clock is noisy (>2× run-to-run). Allocations, not latency, are the
  reliable optimized metric. The cold first-use FTS penalty is diagnosed but the
  first-build is not eliminated.
- **Semantic-vs-lexical quality: NOT TESTED.** A fresh, model-backed ablation was not
  run. Conditional hashed-semantic expansion therefore stays **optional and off by
  default**.
- **Authenticated provenance deployment: BLOCKED.** The mechanism passes adversarial
  tests, but no protected key authority / MCP identity ships. `source:verified` is
  **not** authentication.
- **Scale: NOT TESTED** at 64 clients and 1M/10M memories. Peak RSS ~25–30 MB and
  47 MB binary reported; these are not general benchmarks.
- **Integration: PARTIAL.** Full repository-wide CLI suite, MCP/OpenCode integration,
  and cross-platform behavior were not run to completion.
- **No soak testing.** 24–72 h stability is not measured.

## Known constraints

- The database **must** live on a local filesystem. Synchronized/network mounts are
  ~12× slower at p99 and are only detected + warned.
- Single-writer SQLite ceiling remains; the fix collapses the tail, it does not add
  writers.
- The strict injection filter is conservative and may reject legitimate text.
- Question/bridge parsing is heuristic and not validated across diverse wording.
- `gofmt -l` reports many files on a Windows CRLF checkout — a pre-existing
  environmental artifact, not formatting drift introduced here.

## How to avoid misreading this repo

| If you want to claim… | You may say… |
|---|---|
| "Faster writes under concurrency" | yes — p99 634→48 ms on local ext4 |
| "Fewer allocations in retrieval" | yes — −56.4% allocs/op |
| "Better retrieval quality" | **no** — not measured here |
| "Smarter agents" | **no** — not established |
| "Production ready" | **no** — provenance deployment BLOCKED; scale/integration gaps |
| "Secure" | say "hardened on the tested local threat model; authenticated provenance mechanism PASS, deployment BLOCKED" |

See the [roadmap](roadmap.md) for the work that would close these gaps.
