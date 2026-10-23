# Beacon V7 evidence and verdict

## Scope and provenance

V7 is an **opt-in candidate** on `codex/beacon-v7-dev`, based on the frozen
Phase 5 commit `02e34f98dec1c5d34a2f0d52904969edc39e389f`. Original
Beacon `99c709081ad49da1c5ade205929b61b19360ed9b`, V6 integration
`209a987bc96c29b5e5d811a794d0357f20214463`, and the Phase 5 branch and
evidence were not changed. No code was pushed or deployed. Phase A audit and
design was committed before implementation at `bca7804104b9231c90f752177a8ce68d6918c740`;
see `PHASE_A_AUDIT_AND_DESIGN.md` for the dependency graph, component matrix,
and trace-based root-cause analysis.

The V7 selector and the first held-out fixture were committed together at
`a5f078247a5a36f531d777578d63427893ee6f36` before the first model call on
that fixture. Its exact SHA-256 is
`0c55552721e1ae0dedb979bd9eabdced6e03eeb440ca38e7bb7209d7013ad044`.
Subsequent trust and bridge repairs are separate commits; their reruns on the
same fixture are **regression checks, not fresh validation**.

## Architecture delivered

The default `ListMemories` path and CLI behavior remain compatible. The new
`SearchQuestion` path is selected with `memory list --question-aware --question
<full question>`; `--semantic` explicitly allows offline hashed expansion.
It scopes the project, searches the existing indexed lexical path, discards
instruction-bearing notes, selects directly supporting evidence, and calls
semantic retrieval only when lexical support is absent. Semantic evidence must
support the asked fact or provide a discriminative named link from lexical
evidence. A final trust/conflict pass and whole-memory token budget produce
the agent context and a sufficiency signal. `--semantic` does not turn on ANN
or a neural service. The route and embedding model are emitted in JSON.

This bypasses RRF and its weight knobs on the V7 path and skips vector work on
simple questions. It does **not** delete older APIs, ranking code, or optional
ANN/neural support. It adds deterministic relevance logic, so the total codebase
is not yet smaller. No new dependency was added.

The original-versus-V6 architecture, 22 regressions, 24 shared failures, 54
oracle failures, and the Phase 5 mislabeled semantic arm are documented in the
Phase A audit. The relevant causal observations are that irrelevant extra
evidence triggered A3 abstentions, and a qualified B2 hint was treated as a
definite answer. Semantic retrieval helped B3 two-hop questions but could
introduce unrelated notes. This is why the V7 route starts with precise
lexical evidence and expands only on demand.

## Agent-task comparison

The locked pilot has **56 synthetic task rows** in seven mechanism families,
eight projects per family. These are new tasks and memory histories, but each
family repeats a pattern. This is a pilot, not 56 independent real workflows.
Both arms used the same seeded SQLite store per task, the real original and V7
CLIs, the Phase 5 system prompt/grader, and local `qwen2.5-coder:7b` with
temperature 0 and seed 0. Arm order for model calls was randomized. No gold,
forbidden token, or required evidence ID was shown to the agent.

| Run | Original | V7 | Paired V7 wins/losses | Status |
|---|---:|---:|---:|---|
| Locked first run, candidate `a5f07824` | 40/56 | 51/56 | 11/0 | Fresh pilot for that candidate |
| Trust repair, `133135b4` | 40/56 | 55/56 | 15/0 | Same fixture, post-feedback regression |
| Semantic disabled, `133135b4` | 40/56 | 48/56 | 8/0 | Same fixture, component ablation |
| Trust and bridge repaired, `3f7970d0` | 40/56 | 56/56 | 16/0 | Same fixture, post-feedback regression |

The first-run gains were seven two-hop tasks and four trust tasks. Original
success was 0/8 in both those families; both arms scored 8/8 in direct,
uncertainty, temporal, workflow, and conflict. The initial V7 trust result
was only 4/8: a note with an embedded `developer:` instruction and a
`source:verified` tag could leave a forged factual sentence after sanitization.
Commit `133135b4` makes the answer path reject the entire instruction-bearing
note; its post-feedback rerun scored 8/8 trust tasks. The semantic-off ablation
lost all seven two-hop successes, while preserving the trust successes. Thus
conditional semantic expansion helped these two-hop tasks, but did not help
the other five families in this pilot.

The first-run task-row exact McNemar two-sided p-value is 0.000977. The rows
are clustered by seven repeated mechanisms: only **two of seven families**
improved, with none losing. A family-level sign test gives p=0.5. The task-row
p-value must not be interpreted as statistically credible superiority across
real agent workflows. The first candidate selected the exact required ID set
in 44/56 tasks versus original 16/56; after the trust repair V7 reached
48/56. The remaining eight mismatches were all two-hop tasks with a selected
legacy decoy. One caused an abstention. Commit `3f7970d0` requires a bridge
term to distinguish the correct destination from competing candidates. A new
generic-bucket regression passes; the historical 300-task ID selections were
unchanged. The final candidate's post-feedback rerun scored 56/56 and selected
the exact required ID set in 56/56. This is useful regression evidence, not a
new holdout or proof of generalization.

The historical Phase 5 selection diagnostic is development evidence: V7
selected exactly the required ID set in A3 20/20, B2 20/20, B3 20/20, B5
20/20, and C3 20/20. The separate historical 80-task model pilot was 67/80
for V7 versus 54/80 for original. Those data informed the design and cannot
be presented as fresh validation. The Phase 5 300-task outcome remains original
229/300 versus V6 intelligent 224/300; V7 has **not** run a fresh 300-task
real-workflow evaluation.

## Latency, tokens, scalability

In the first locked run, mean retrieval latency was 18.1 ms for original and
48.1 ms for V7; mean end-to-end model plus retrieval was 0.455 s and 0.516 s.
Mean prompt tokens were 137.2 and 131.7. These are small, cold CLI runs, not
steady-state serving benchmarks. The final post-feedback rerun averaged
0.585 s end-to-end versus 0.483 s original, but overlapped a race-enabled Go
test and is unsuitable for a clean latency comparison. V7 has **not** shown
the requested speed improvement. The semantic-off ablation averaged 38.0 ms
retrieval versus 11.4 ms original in that separate run; it establishes no
overall speed advantage either.

The V7 selector used lexical only on 40/56 pilot tasks and semantic on 16/56.
On-demand embedding backfill is capped at 1,000 project memories and fails
explicitly above that cap; a lexical writer invalidates stale derived vectors.
ANN remains opt-in and was not needed by this pilot. Existing ANN correctness
tests are part of the Go learning suite; the user-requested 1M benchmark was
not run. No claim is made for neural-in-agent-loop quality or large-scale V7
memory use.

A matched five-second, 16-client mixed writer probe on WSL2 found zero errors
but write p99 of **742 ms** for V7 versus **635 ms** for frozen Phase 5, with
about 2,075 versus 2,203 writes. Both exceed a practical low-latency writer
gate. Short runs are noisy; this does not prove V7 caused the difference. The
architecture still uses SQLite's single writer and the 16-client tail remains
a release blocker. Peak RSS in these probes was 29 MiB versus 28 MiB; this is
not a general memory-consumption benchmark.

## Tests and operational state

- V7-focused unit tests cover direct lexical routing, conditional semantic
  links, trust/scope, uncertainty, budget integrity, backfill and stale-vector
  invalidation. A new test covers verified-tag prompt injection; another
  covers a generic shared bridge word that linked a decoy. Focused tests and
  `go vet` passed after both repairs.
- Full `internal/learning` and `cmd` package suites passed after the first
  candidate. The full `go test ./...` run passed at commit `133135b4` from a
  clean LF checkout in WSL2, excluding
  `TestMCPConnectEndToEndOpenCode`. That single test failed identically on
  untouched Phase 5 with “No MCP servers configured.” The full Windows-mounted
  checkout run failed fixture-text tests in unrelated endpoint/pricing packages;
  those packages passed from the LF checkout and from frozen Phase 5.
- Race-enabled crash, scope, and V7 question tests passed, including process
  crash injection at transaction, index, migration, and supersession points.
  The historical 300-task selection diagnostic after the bridge repair passed
  and produced identical ID lists to the earlier V7 diagnostic. At final code
  commit `3f7970d0`, the full learning package, repository-wide `go vet`, the
  V7 CLI integration test, and a race-enabled crash/scope/question subset
  passed from the clean WSL checkout.

## Remaining defects and verdict

V7 is a promising **opt-in research candidate**, not a default or release-ready
replacement. The first fresh pilot shows a task-success improvement, and the
targeted semantic ablation supports a concrete two-hop benefit. It does not
demonstrate overwhelming or statistically credible superiority over original
Beacon on diverse real agent work. Its measured cold end-to-end latency is
slower. The 16-client writer tail remains high. Source tags are not
cryptographically authenticated; the strict injection filter is conservative
and may reject legitimate text. Question and bridge parsing is heuristic and
has not been validated across diverse wording. Contradiction success was
8/8 for both arms in the pilot, so no improvement is shown. Neural retrieval,
ANN at agent level, long-term cache behavior, mixed-size memory use, and
real-workflow task success remain unproven.

Release gates: (1) an untouched, diverse **real workflow** holdout comparing
original, V6, and final V7 with cluster-aware confidence intervals and full
failure analysis; (2) end-to-end latency at least comparable to original on
the same unloaded hardware; (3) sustained 16-client writer p99 within the
chosen budget with zero durability loss; (4) authenticated provenance and
adversarial trust checks beyond synthetic tags; (5) a separate neural/ANN
quality and operational gate before either is enabled by default. No paid
compute, remote push, PR, or deployment is needed for the next local checks.

## Reproducibility manifest

- WSL2 host: Linux x86-64, eight visible CPUs, 7.6 GiB RAM; Go 1.27 toolchain.
  Local Mac: `Mac17,3`, 16 GiB RAM, macOS 26.6.2; Ollama 0.40.6,
  `qwen2.5-coder:7b`. No paid compute.
- Original CLI SHA-256:
  `a486ef9f9c1bdc6c5dd9c039aff5c38881ef6a68c63922c27a6f0d29963b3592`.
  Seed helper:
  `ebcf6aceee9d14911d73b593ca68e3863670e769737422b16aedf76942e0452d`.
  First V7 CLI:
  `2229ea756caa3b7a8b37a34ab58e05264b43b4a126dffd407a83f78da4c94489`.
  Trust-repaired CLI:
  `f25b8b6df59b97e562d3dd8753d482ed2ebd9c3c3622429328527282f07a110a`.
  Bridge-repaired CLI:
  `24d1e8aa6892d44c5e395dde7406d415c76cf3b2d1231c5274b6d355e1d59078`.
- First locked raw pilot SHA-256:
  `7d42bae6b86c0abc2b79938db54da7faa381ef3141c0da4cb3ceb75dbc4d1e0b`.
  Trust-repair rerun:
  `eb585b0175ccceee045a4c364a57026fe84006a53c941ddeefb40d13fee503a5`.
  Semantic-off ablation:
  `625299a304d59e8396e70b3fa94e3fe03ef2e1fb19d94e9b058b8683cf984e14`.
  Final bridge-repair regression:
  `977b9a3891063bf954145c0f3e1220a2f2b2a9d014a72822c090a1beb2b43028`.
  Raw JSON and test logs are under `validation/raw/`; the historical development
  files are under `dev/raw/`. `validation/LOCK.md` and `run_pilot.py` give the
  fixture lock and exact command-line protocol.
