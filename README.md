<p align="center">
  <img src="images/beacon-lockup-light-mode.svg#gh-light-mode-only" alt="Beacon" width="480">
  <img src="images/beacon-lockup-dark-mode.svg#gh-dark-mode-only" alt="Beacon" width="480">
</p>

<h1 align="center">Self‑improving memory for coding agents</h1>

<p align="center">
  <a href="https://github.com/asymptote-labs/agent-beacon/releases"><img src="https://img.shields.io/github/v/release/asymptote-labs/agent-beacon" alt="GitHub release"></a>
  <a href="https://github.com/asymptote-labs/homebrew-tap"><img src="https://img.shields.io/badge/homebrew-beacon-fbb040?logo=homebrew" alt="Homebrew"></a>
  <a href="https://github.com/asymptote-labs/agent-beacon/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/asymptote-labs/agent-beacon/ci.yml" alt="GitHub Workflow Status"></a>
  <a href="https://github.com/asymptote-labs/agent-beacon/blob/main/LICENSE"><img src="https://img.shields.io/github/license/asymptote-labs/agent-beacon" alt="MIT license"></a>
  <a href="https://docs.beacon.sh"><img src="https://img.shields.io/badge/docs-beacon.sh-0369a1" alt="Docs"></a>
  <a href="https://discord.gg/zdNChS2fBu"><img src="https://img.shields.io/badge/discord-community-5865F2?logo=discord&logoColor=white" alt="Discord"></a>
</p>

<p align="center">
  <a href="https://beacon.sh">Website</a>
  ·
  <a href="https://docs.beacon.sh">Docs</a>
  ·
  <a href="https://discord.gg/zdNChS2fBu">Discord</a>
  ·
  <a href="https://docs.beacon.sh/cli/install">Install</a>
  ·
  <a href="https://docs.beacon.sh/cli">Commands</a>
</p>

<p align="center">
  <a href="https://www.star-history.com/asymptote-labs/agent-beacon">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/badge?repo=Asymptote-Labs/agent-beacon&type=trending&theme=dark">
      <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/badge?repo=Asymptote-Labs/agent-beacon&type=trending">
      <img alt="GitHub Trending Repository of the Day" src="https://api.star-history.com/badge?repo=Asymptote-Labs/agent-beacon&type=trending">
    </picture>
  </a>
</p>

Beacon captures **agent session history** across Claude Code, Cursor, Codex, OpenCode, and 20+ other harnesses, then turns useful workflows, corrections, and debugging patterns into reusable knowledge for future agents.

**A problem solved by one agent shouldn't need to be learned from scratch by another.**

---

## Beacon Overview

Beacon is an open-source memory layer for AI coding agents that learns from your work across Claude Code, Cursor, Codex, OpenCode, and 20+ other harnesses. It captures full session history, identifies useful workflows, corrections, and debugging patterns, and makes that knowledge reusable by future agents. Built for developers who want agent knowledge to compound across tools instead of disappearing when a session ends.

**Key Capabilities:**

- **Cross-harness history** - sessions from Claude Code, Cursor, Codex, OpenCode, Cline, and 20+ harnesses in one place
- **Knowledge that compounds** - workflows, corrections, debugging patterns, and repo conventions that survive beyond a single session
- **Shared agent memory** - reviewed knowledge future agents can retrieve through MCP or Agent Skills
- **Exact session replay** - prompts, responses, tools, commands, edits, approvals, MCP activity, and tokens in one trace
- **Local-first portability** - durable JSONL, explicit destinations, and no harness lock-in

---

## 🧩 Memory-Engine Optimization (this lineage)

> **Attribution.** Agent Beacon is an open-source project by
> [Asymptote Labs](https://github.com/Asymptote-Labs/agent-beacon) (MIT). This repository
> is that project **plus** a body of memory-engine optimization and hardening commits
> (everything after `7b605aa84`). Upstream product code, docs, and license notices are
> preserved unchanged. The optimization work is documented and evidenced in
> [`docs/optimization/`](docs/optimization/index.md) and [`reports/`](reports/).

The optimization changes **three things** in the memory engine and leaves the product
interfaces and defaults intact. All numbers below are **reproducible** and point to a
source report; nothing here claims an agent-quality improvement.

### Performance at a glance

| Metric | Baseline | Optimized | Source |
|---|---:|---:|---|
| 16-client write p99 (local ext4, 8 s mixed) | 634 ms | **48–54 ms** | [`D_DATABASE_CONCURRENCY_FIX.md`](reports/v7-hardening/D_DATABASE_CONCURRENCY_FIX.md) |
| Question-aware retrieval allocs/op (warm, 1k store) | 2,007 | **875 (−56.4%)** | [`RETRIEVAL_ALLOCATION_FIX.md`](reports/v7-hardening/RETRIEVAL_ALLOCATION_FIX.md) |
| Question-aware retrieval bytes/op | 208,054 | **130,495 (−37.3%)** | [`RETRIEVAL_ALLOCATION_FIX.md`](reports/v7-hardening/RETRIEVAL_ALLOCATION_FIX.md) |
| Indexed no-match search (scaling with N) | O(N): 8.05→46 ms | **81→142 µs** (flat) | [`security/00_EXECUTIVE_SUMMARY.md`](reports/security/00_EXECUTIVE_SUMMARY.md) |
| Cold healthy-store retrieval | 43.7 ms | **20–36 ms** (≈ original) | [`FINAL_STABILIZATION_REPORT.md`](reports/v7-hardening/FINAL_STABILIZATION_REPORT.md) |

Workload, hardware (WSL2, 8 vCPU, 7.6 GiB, Go 1.27), comparator commits, and units are in
[`docs/optimization/performance.md`](docs/optimization/performance.md). These are
**storage/retrieval microbenchmarks**, not agent-task outcomes.

### What changed

1. **Write concurrency** — a process-level mutex serializes in-process writers instead of
   letting 16 SQLite connections busy-poll the single write lock. Durability is
   unchanged (WAL, `synchronous` untouched); the p99 tail collapses.
2. **Retrieval allocations** — significant-term sets are computed once and shared across
   supersession/contradiction detection, and intersection counts no longer allocate.
3. **Placement + lifecycle** — a configurable local database directory
   (`BEACON_MEMORY_DIR`) with synchronized-mount detection and safe migration, plus
   project authorization, atomic lifecycle, bounded search, and an opt-in Ed25519
   provenance mechanism.

### Try it (no model, no GPU, ~5 minutes)

```sh
# Build the CLI
cd cli/beacon && CGO_ENABLED=0 go build -o /tmp/beacon .
export BEACON_BIN=/tmp/beacon

# Example A: store a memory and retrieve it through the real MCP server
cd ../../examples/memory-basic && python3 run.py
```

See [`examples/`](examples/) for scale retrieval and agent/MCP integration examples.

### Reproduce the headline benchmark

```sh
cd cli/beacon
BEACON_PHASE3_WRITER=1 BEACON_PHASE3_W_CONCURRENCY=16 BEACON_PHASE3_W_SECONDS=8 \
  go test ./internal/learning/ -run TestPhase3BWriterLatency -v -count=1
```

Full instructions: [`docs/optimization/benchmarks.md`](docs/optimization/benchmarks.md).

### Honest status

Reproducible today: the write-concurrency fix, the allocation reduction, cold-start
parity on healthy stores, safe placement/migration, and core/race/vet/build. **Open or
blocked:** authenticated-provenance *deployment* (no protected key authority ships),
semantic-vs-lexical quality (fresh ablation not run), 64-client/1M scale, full
integration, and any agent-intelligence claim. Read
[`docs/optimization/limitations.md`](docs/optimization/limitations.md) before quoting
anything. This build is **not** certified production-ready.

---

## 🎬 Demo

https://github.com/user-attachments/assets/38995612-84e9-4989-a905-0099f77955f0

---

## 🚀 Quick Start

Beacon is open source and local-first. Interactive endpoint setup signs in through
beacon.sh and preselects Beacon Cloud, with an explicit Local opt-out. Signing in
forwards nothing; confirming Managed installs Beacon and connects this machine in
the same command, and the confirm screen says so before you accept. Setup then ends
on a short summary and opens the hosted dashboard at
[beacon.sh/dashboard](https://beacon.sh/dashboard). System, package, MDM, and CI
installation paths remain noninteractive and account-free.

### 1. Install Beacon

**Copy & paste setup prompt into your coding agent**

```text
Read https://beacon.sh/start.md then set me up on Beacon
```

<details open>
<summary><strong>macOS</strong></summary>

```bash
brew update
brew trust asymptote-labs/tap
brew tap asymptote-labs/tap
brew install beacon

beacon endpoint install
```

</details>

<details>
<summary><strong>Linux</strong></summary>

Install the latest `.deb` or `.rpm` for your distribution and architecture (Debian, Ubuntu, Fedora, RHEL, Rocky, Alma):

```bash
curl -fsSL https://github.com/asymptote-labs/agent-beacon/releases/latest/download/install.sh | bash
```

The [installer](packaging/linux/install.sh) downloads the package for your architecture, verifies it against the release `checksums.txt`, and installs it with `apt-get` or `dnf`. Set `BEACON_VERSION=<version>` to install a specific release.

**Prefer not to pipe a script into your shell?** Either read the script before you run it:

```bash
curl -fsSLO https://github.com/asymptote-labs/agent-beacon/releases/latest/download/install.sh
less install.sh
bash install.sh
```

Or skip the script and run its steps yourself: pick a version, download the package and `checksums.txt`, verify the package, and install it with your package manager.

```bash
VERSION="$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/asymptote-labs/agent-beacon/releases/latest)"; VERSION="${VERSION##*/v}"
case "$(uname -m)" in x86_64) ARCH=amd64 ;; aarch64) ARCH=arm64 ;; esac
FORMAT=deb   # or rpm for Fedora, RHEL, Rocky, Alma
PKG="beacon_${VERSION}_linux_${ARCH}.${FORMAT}"
BASE="https://github.com/asymptote-labs/agent-beacon/releases/download/v${VERSION}"

# Download into a directory APT can read. A package in your home directory still
# installs, but APT ends with a harmless "Permission denied" notice.
DIR="$(mktemp -d /tmp/beacon-install.XXXXXXXX)" && chmod 0711 "$DIR" && cd "$DIR"
curl -fsSLO "${BASE}/${PKG}" && curl -fsSLO "${BASE}/checksums.txt" && chmod 0644 "${PKG}"
grep "  ${PKG}$" checksums.txt | sha256sum --check - && \
sudo apt install "./${PKG}"   # or: sudo dnf install "./${PKG}"
```

`checksums.txt` is published with the release, so it catches a corrupted or swapped download but does not prove who built the release. To check that, verify the package's build-provenance attestation, which shows it was built by this repository's release workflow: `gh attestation verify "${PKG}" --repo asymptote-labs/agent-beacon`. You can also pin `VERSION` to a release you have reviewed instead of following `latest`. To install without root, use the tarball in user mode ([Linux install guide](https://docs.asymptotelabs.ai/platforms/linux#without-root-or-without-a-package-manager)).

</details>

<details>
<summary><strong>Windows</strong></summary>

Download the x64 MSI from the [latest release](https://github.com/asymptote-labs/agent-beacon/releases/latest).

```bash
msiexec /i BeaconEndpointAgent-<version>-x64.msi
```

For silent installation:

```bash
msiexec /i BeaconEndpointAgent-<version>-x64.msi /qn
```

</details>

### 2. Use your agents normally

Open Claude Code, Cursor, Codex, or any other supported harness.

Beacon continuously captures your session history in the background

### 3. Explore your history

```bash
beacon traces
```

This opens a local terminal browser for traces, event timelines, token usage, and retained content. Nothing is sent anywhere. To use the local web view instead:

```bash
beacon endpoint dashboard
```

Or inspect the raw event stream:

```text
~/.beacon/endpoint/logs/runtime.jsonl
```

> [!NOTE]
> Signing in does not enable forwarding. Confirming the preselected Beacon Cloud
> option does: the confirm screen names the signed-in account, states that
> confirming installs Beacon and starts forwarding, names Asymptote Labs as the
> operator, the destination, and the capture mode, and connects the endpoint
> after the install succeeds. Choose Local to keep everything on this machine,
> and disconnect any time with `beacon endpoint disconnect`.

Whichever destination you choose, the wizard also installs the
[Beacon Skills](https://docs.beacon.sh/concepts/beacon-skills) into `~/.agents/skills`
and `~/.claude/skills` unless you turn them off; redo or undo that with
`beacon skills install|uninstall`.

If you choose Beacon Cloud, the wizard also registers Beacon Cloud MCP in your harnesses
unless you turn it off, so your agents can search the forwarded history. To do it later,
or for an endpoint connected another way:

```bash
beacon mcp connect
```

This registers the `beacon-managed` MCP server in Claude Code, Codex CLI, Cursor, VS Code, Gemini CLI, and OpenCode, and each harness signs in with OAuth the first time it connects. No token is written. `beacon mcp disconnect` removes exactly what it added. The local `beacon` MCP server (`beacon mcp serve`) is separate and never touches the network. See [`beacon mcp connect`](https://docs.beacon.sh/cli/mcp-connect).

Inspect the account used during interactive setup:

```bash
beacon whoami
```

---

## 🧠 Turn Session History Into Memory

Every agent session contains potentially useful knowledge about your codebase.

Beacon creates a loop around that history:

```text
Run agents
    ↓
Capture session history
    ↓
Evaluate what worked
    ↓
Extract useful knowledge
    ↓
Review + approve
    ↓
Reuse across future agents
```

That could be:

- the right way to run a migration
- a debugging path that finally fixed an obscure issue
- a testing convention agents repeatedly get wrong
- a repository-specific workflow
- the right sequence of internal tools
- a correction you've given multiple agents

Instead of disappearing into old sessions, that knowledge becomes reusable.

---

## 🔀 Cross-Harness by Design

Most agent memory belongs to a single harness.

Beacon sits across the harness layer.

```text
Claude Code ─┐
Cursor ──────┤
Codex ───────┼──→ Beacon ──→ shared project knowledge
OpenCode ────┤
Cline ───────┘
```

Because Beacon captures and normalizes session history across tools, knowledge learned through Claude Code doesn't have to stay in Claude Code.

Your Cursor sessions can improve Codex.  
Your Codex sessions can improve OpenCode.  
Your history keeps compounding even as you switch tools.

**Your agent session history belongs to you, not the harness.**

---

## 🔎 One Trace Format for Every Agent

Beacon captures agent execution where it happens and normalizes it into a common OpenTelemetry-based event model.

That includes:

- sessions
- prompts and responses
- tool calls
- commands
- file activity
- approvals
- MCP interactions
- token usage

Instead of separate proprietary histories for every coding tool, you get one dataset you can inspect, search, learn from, and build on.

---

## 🖥️ Local Dashboard

Beacon ships with a local, read-only dashboard:

```bash
beacon endpoint dashboard
```

Use it to explore session history across harnesses and understand what your agents actually did.

Each session page can also show **lenses**: purpose-built views of one trace, such as files
changed with their diffs, an itemized token bill, a security review grouped by risk, or the
approval timeline. A lens is a single HTML file running in a sandboxed frame with no access to
Beacon or the dashboard and no ordinary network APIs, so you can have your coding agent write
your own (`beacon lenses spec`, `lint`, `preview`, `add`). The sandbox cannot block every way a
browser can send data out, so install only lenses you trust with your traces. See
[Lenses](https://docs.beacon.sh/concepts/lenses).

The underlying JSONL remains directly accessible, so you're never dependent on the UI.

---

## Supported Agents

Beacon supports local agents, browser agents, cloud agents, CI workflows, and agent SDKs.

### Local Agent Coverage

| Runtime | Collection | Session | Prompt | Tool | Command | File | Approval | MCP | Tokens | Skills |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Antigravity CLI | Hooks | ✅ | ✅ | ✅ | ✅ | ✅ | – | – | – | ✅ skills CLI |
| Claude Code | OTLP + hooks + poll | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ Plugin |
| Claude Cowork | OTLP | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ Plugin |
| Cline | Plugin + poll | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ | ✅ `.cline/skills` |
| Codex CLI | OTLP + hooks + poll | ✅ | ✅ | ✅ | ✅ | – | ✅ | – | ✅ | ✅ Plugin |
| Codex Desktop | OTLP | ✅ | ✅ | ✅ | ✅ | – | ✅ | – | ✅ | ✅ Plugin |
| Cursor | Hooks + poll | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ Plugin |
| DeepSeek Harness | Hooks + poll | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ | ✅ `.agents/skills` |
| Devin CLI | Hooks | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ Plugin |
| Devin Desktop | Hooks | – | ✅ | ✅ | ✅ | ✅ | – | ✅ | – | ✅ via Devin CLI |
| Factory Droid | OTLP + hooks + poll | ✅ | ✅ | ✅ | – | ✅ | ✅ | – | – | ✅ Plugin |
| fx (Vercel Labs) | Poll | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ | ✅ `/skills install` |
| Gemini CLI | OTLP | – | ✅ | ✅ | – | ✅ | ✅ | ✅ | – | ✅ `gemini skills install` |
| GitHub Copilot CLI | OTLP + poll | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ Plugin |
| goose | Adapter only; manual hooks/OTLP | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ | ✅ skills CLI |
| Grok Build | Hooks + poll | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | – | – | ✅ Plugin |
| Hermes Agent | Hooks + poll | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ `hermes skills install` |
| Kimi Code | Hooks | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ Plugin |
| Kiro | Hooks | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ | – | ✅ Power |
| Muse Code | Hooks | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | – | – | ✅ `.agents/skills` |
| Oh My Pi | Extension | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ Plugin |
| OpenClaw Gateway | Plugin + OTLP + poll | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ | ~ | ✅ Plugin |
| OpenCode | Plugin + poll | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ skills CLI |
| OpenHands | Hooks | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ | – | ✅ skills CLI |
| Pi | Extension + poll | ✅ | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ | ✅ Package |
| Prime Agent | Extension + poll | ✅ | ✅ | ✅ | ✅ | ✅ | – | – | ✅ | ✅ Package |
| Qwen Code | Hooks | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | – | – | ✅ Plugin |
| Senpi | Extension | ✅ | ✅ | ✅ | ✅ | ✅ | – | – | ✅ | ✅ `.agents/skills` |
| VS Code | OTLP + hooks | ✅ | ✅ | ✅ | ~ | ~ | – | ~ | – | ✅ Plugin |

**Skills** shows how to install [Beacon Skills](https://docs.beacon.sh/concepts/beacon-skills), the Agent Skills that recall and distill project memory from these traces. Every runtime above loads Agent Skills:

- **Plugin**: install the `beacon` plugin from this repository's marketplace, for example `/plugin marketplace add asymptote-labs/agent-beacon` in Claude Code.
- **Package** (Pi, Prime Agent): `pi install git:github.com/asymptote-labs/agent-beacon`.
- **Power** (Kiro): import `https://github.com/asymptote-labs/agent-beacon/tree/main/agent-skills` from the Powers panel.
- **skills CLI**: `npx skills add asymptote-labs/agent-beacon`, optionally with `-a <agent>`.
- **A directory**: copy `agent-skills/skills/*` into that project directory.

The [Beacon Skills page](https://docs.beacon.sh/concepts/beacon-skills#install) has the exact command for each runtime.

### Browser Chat

| Site | Collection | Prompt | Response | Tool | Tokens |
| --- | --- | --- | --- | --- | --- |
| Claude.ai | Chromium + Firefox extension → local OTLP | ✅ | ✅ | ✅ | ~ |
| ChatGPT | Chromium + Firefox extension → local OTLP | ✅ | ✅ | ✅ | – |

### Cloud Agents

| Runtime | Collection | Session | Prompt | Tool | Command | File | Tokens |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Claude Code Cloud Agents | Sandbox hooks → GCS or S3 | ✅ | ✅ | ✅ | ✅ | ✅ | – |
| Cursor Cloud Agents | Sandbox hooks → GCS or S3 | – | ✅ | ✅ | ✅ | ✅ | – |
| Devin Cloud Agents | API poll → GCS | ✅ | ✅ | – | – | – | ✅ |
| CI jobs | `beacon ci exec` → temporary local collector | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

### SDK Instrumentation

| SDK Surface | Collection | Captures |
| --- | --- | --- |
| Anthropic | OpenLLMetry through `@asymptote/sdk` | Model call spans, errors, and OTel attributes |
| OpenAI | OpenLLMetry through `@asymptote/sdk` | Model call spans, errors, and OTel attributes |
| Claude Agent SDK | `Observe.wrapClaudeAgentQuery()` | Query root spans with Beacon-compatible prompt attributes |
| Vercel AI SDK | `experimental_telemetry` tracer handoff | Model call and tool spans where telemetry is enabled |

---

## Output Destinations

Beacon writes endpoint telemetry to local JSONL by default.

You can also forward the same normalized session history into infrastructure you already use:

**Splunk · Datadog · Elastic · Microsoft Sentinel · CrowdStrike Falcon LogScale · Sumo Logic · Wazuh · AWS S3 · GCS · CloudWatch**

| Destination | Category | Support Path |
| --- | --- | --- |
| Local JSONL | Local | Default endpoint log and local dashboard source |
| Beacon Cloud | Hosted forwarding | Signed-in device enrollment with Standard or Metadata-only privacy |
| CrowdStrike Falcon LogScale HEC | SIEM | Endpoint forwarding with LogScale ingest tokens |
| Microsoft Sentinel | SIEM | Azure Monitor Agent and Data Collection Rule content pack |
| Rapid7 InsightIDR | SIEM | Custom Logs webhook content pack |
| Splunk HEC | SIEM | Endpoint forwarding during install or repair |
| Sumo Logic | SIEM | HTTP Logs & Metrics Source content pack |
| Wazuh | SIEM | Localfile configuration and Beacon content pack |
| AWS CloudWatch Logs | Log aggregation | Vector content pack |
| Datadog | Log aggregation | Datadog Agent custom log collection |
| Elastic | Log aggregation | Filebeat or Elastic Agent |
| Customer-managed pipelines | Log aggregation | Forward directly from local Beacon JSONL |
| AWS S3 | Object storage | Vector, CI upload, or cloud-agent snapshots |
| Google Cloud Storage | Object storage | Vector, CI upload, or cloud-agent snapshots |

---

## Architecture

Beacon captures activity where agents actually run and normalizes it into one shared event model.

```text
Local agents ───────┐
Browser chat ───────┤
CI ─────────────────┼──→ Beacon ──→ unified session history
Cloud agents ───────┤                    │
Agent SDKs ─────────┘                    ├──→ local JSONL
                                         ├──→ reviewed memory
                                         ├──→ MCP / Agent Skills
                                         └──→ your own infrastructure
```

See the [documentation](https://docs.beacon.sh/architecture/architecture) for the full architecture breakdown.

---

## Documentation

Read the docs for:

- installation
- supported runtimes
- event schema
- session history
- memory
- MCP
- Agent Skills
- forwarding
- advanced configuration

[**Read the docs →**](https://docs.beacon.sh)

### Memory-engine optimization docs (this lineage)

- [Overview](docs/optimization/index.md)
- [Architecture](docs/optimization/architecture.md) · [Retrieval pipeline](docs/optimization/retrieval-pipeline.md)
- [Performance](docs/optimization/performance.md) · [Benchmarks & reproduction](docs/optimization/benchmarks.md)
- [Security](docs/optimization/security.md) · [Configuration](docs/optimization/configuration.md)
- [Limitations](docs/optimization/limitations.md) · [Roadmap](docs/optimization/roadmap.md)
- [Troubleshooting](docs/optimization/troubleshooting.md)
- [Examples](examples/README.md) · [Evidence reports](reports/)

---

## Contributing

Contributions are welcome.

Open an issue, submit a pull request, or join the [Discord](https://discord.gg/zdNChS2fBu).

See [`CONTRIBUTING.md`](CONTRIBUTING.md). For the memory-engine optimization work,
start with [`docs/optimization/`](docs/optimization/index.md) and mind the rules there:
evidence or it didn't happen, never claim agent-quality gains from a retrieval
microbenchmark, and preserve the default path.

---

## License

[MIT](LICENSE)

## Star History

<a href="https://www.star-history.com/?repos=asymptote-labs%2Fagent-beacon&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=asymptote-labs/agent-beacon&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=asymptote-labs/agent-beacon&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=asymptote-labs/agent-beacon&type=date&legend=top-left" />
 </picture>
</a>
