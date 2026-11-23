# Beacon V6 — Phase 4D Executive Results

Final integrated optimization of the V6 Intelligent Memory system: one consistent
post-retrieval decision layer, optimized, hardened, and contract-tested.

- Date: 2026-10-23 (Asia/Kolkata)
- Branch: `phase4d/beacon-integrated-optimization` (worktree `/home/<user>/work/phase4d`)
- Base: Phase 4C `12226058ac0ba57c6e0e01a71356bc83abd81735`
- Frozen refs unchanged: `99c70908…`, `bcf3a1fd…`, `e032ee73…`, `2932e8e5…`, `12226058…`
- Hosts: WSL2 laptop (8 vCPU, 7.6 GiB, Go 1.27.3) + Apple M5 Mac (`192.0.2.10`, MPS, Ollama 0.40.6)
- No paid compute, no downloads, no push/PR.

**Bottom line.** Phase 4D turns the Phase 4C prototype into a faster, safer, reachable
system **without regressing correctness**. The context selector is now near-linear
(**31.5× faster at n=1000**, identical results), the unresolvable-ambiguity over-answering
is **fixed** (ambiguity 7/8 → **8/8**), the intelligent arm reaches **exact oracle parity**
on the dev set (27/35, paired **0–0–35**, p=1.0) with **7–0** over raw semantic retrieval
(p=0.0156), prompt-injection hardening now covers encoded/obfuscated payloads, and the
whole stack is reachable through a real CLI opt-in (`memory list --semantic --intelligent`).
A 12-test cross-component contract suite guards every mechanism interaction. Defaults are
unchanged; all intelligence remains opt-in.

---

## 1. Summary matrix

| # | Problem | Root cause | Change | Correctness before→after | Performance before→after | Security | Integration | Gate |
|---|---|---|---|---|---|---|---|---|
| 1 | Selector O(n²) pairwise scan | global pair loop | inverted-index candidate discovery | identical (equality test) | 168 ms→5.4 ms @ n=1000 (31.5×) | — | internal | **PASS** |
| 2 | Over-answer on unresolvable ambiguity | no sufficiency signal | structured sufficiency + light stemming | ambiguity 7/8→**8/8** | negligible | — | CLI `--sufficiency` | **PASS** |
| 3 | Weak injection coverage | keyword-only | + base64/zero-width detection | attacks 4/6→0/6 (dev) | negligible | improved | internal | **PASS** |
| 4 | Lexical supersession only | keyword markers | structured `SupersededBy` + chains + cycle break | chains resolve deterministically | — | — | internal | **PASS** |
| 5 | Layer unreachable in production | never called by CLI/MCP | opt-in `--semantic --intelligent` | defaults unchanged | — | — | CLI/MCP-safe | **PASS** |
| 6 | Mechanisms could conflict | no interaction tests | 12 contract tests | all pass | — | — | cross-component | **PASS** |
| 7 | 16-client write p99 | SQLite single writer | not changed (durability) | unchanged | documented | — | storage | **CONDITIONAL** |
| 8 | Token-budget matrix | not run | — | — | — | — | — | **BLOCKED** |

## 2. Before/after (headline)

### Context-selector scaling (same machine, same corpus)

| n | naive O(n²) detect | indexed detect | speedup |
|---:|---:|---:|---:|
| 20 | 0.21 ms | 0.19 ms | 1.1× |
| 100 | 2.15 ms | 0.88 ms | 2.5× |
| 500 | 38.5 ms | 3.9 ms | 9.9× |
| 1000 | 168.4 ms | 5.4 ms | **31.5×** |

`BenchmarkSelectTrustContext1000`: 9.0 ms/op, 3.15 MB/op, 27,421 allocs/op (was ~1.5 s
with the O(n²) path).

### End-to-end dev pilot (35 cases, qwen2.5-coder:7b)

| Arm | Phase 4C | Phase 4D |
|---|---:|---:|
| none | 6/35 | 6/35 |
| original | 6/35 | 6/35 |
| v6_semantic | 20/35 | 20/35 |
| **v6_intelligent** | 26/35 | **27/35 (oracle parity)** |
| oracle | 27/35 | 27/35 |

Paired (Phase 4D): v6_intelligent vs v6_semantic **7–0–28, p=0.0156**; vs original
**21–0–14, p<1e-4**; vs oracle **0–0–35, p=1.0**.

### Held-out (18 frozen cases) — no regression

| Arm | Phase 4C | Phase 4D |
|---|---:|---:|
| v6_semantic | 10/18 | 10/18 |
| v6_intelligent | 14/18 | 14/18 |
| oracle | 15/18 | 15/18 |

## 3. Verification

| Check | Result |
|---|---|
| `gofmt -l` | clean |
| `go build ./...` | pass |
| `go vet ./internal/learning/ ./cmd/` | clean |
| Full learning suite | **ok 67.8 s** |
| `go test -race` subset | **ok 42.9 s** |
| Layer unit tests (8) + contract tests (12) + history test | PASS |
| Phase 3 locked fixtures | `LOCK OK ×3` (unchanged) |
| `cmd` memory tests | ok |

## 4. Answers to the ten questions

1. **Most real tasks correct:** v6_intelligent (27/35, oracle parity).
2. **Best contradiction detection:** unchanged detector (locked P=1.0/R=0.50 default,
   cross-kind opt-in 1.0); now runs over candidate pairs (same results, faster).
3. **Best applicable-memory choice:** v6_intelligent (structured supersession + trust).
4. **New corrections vs misleading updates:** v6_intelligent (false-newer rejected).
5. **Ambiguity:** v6_intelligent **8/8** (fixed over-answering).
6. **Fewest unsafe/unsupported decisions:** v6_intelligent (0 injection successes;
   sufficiency signal prevents guessing).
7. **Accurate as histories/corpus grow:** selector near-linear; history PASS; 1M **BLOCKED**.
8. **Best latency/cost per success:** v6_intelligent (fewer tokens, faster selection).
9. **Beats V6 semantic on unseen tasks:** held-out 4–0 (14 vs 10), not significant at n=18
   — **CONDITIONAL**; Phase 5 will decide.
10. **Biggest remaining bottleneck:** model reasoning ceiling + the 16-client write tail.

## 5. Verdict

**Yes — the optimized system is more accurate (27/35, oracle parity), more reliable
(12/12 contract tests, no regressions), more secure (0/6 dev and 0/4 held-out injection
success, + encoded-payload detection), and more efficient (31.5× selector speedup, lower
tokens) than the previous V6 implementation, without introducing contradictory behavior.**
The held-out superiority over V6 semantic remains **CONDITIONAL** (directional, n=18) and
is deferred to the Phase 5 campaign. The 16-client write tail and 1M scale remain open.

## 6. Artifacts

- Reports: `reports/phase4/part4/*.md`
- Raw evidence: `reports/phase4/part4/RAW_EVIDENCE/`
- Implementation: `cli/beacon/internal/learning/context_select.go` (+ contract, scaling,
  history tests), `cli/beacon/cmd/memory.go` (opt-in flags)
- Reproduction: `reports/phase4/part4/run_phase4d.sh`
