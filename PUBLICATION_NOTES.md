# Publication notes

This repository is a **sanitized publication copy** of the Agent Beacon memory-engine
optimization work. It is derived from an internal development checkout and prepared for
public release.

## What was redacted

Personal identifiers were replaced with neutral placeholders in the report and
documentation text:

- contributor name → `<user>`
- machine hostname → `<host>`
- local LAN addresses `192.168.1.x` → `192.0.2.x` (TEST-NET-1 documentation range)
- home/workspace paths (`/home/<user>`, `C:\Users\<user>\`) → neutral placeholders
- private bundle references → `upstream-history.bundle`

The redaction changed **only** identities and paths. All benchmark numbers, units,
methodology, findings, comparator commit SHAs, and reproducibility evidence are
preserved verbatim.

## What was excluded (large binary artifacts)

To keep the repository a reasonable size, the following raw binary artifacts were
**removed** from the published tree. Their content is still described by the committed
human-readable summaries (`*_top.txt`, `*.json`) and the report prose:

| Removed artifact | Size | Where its result lives now |
|---|---:|---|
| `cli/beacon/beacon.zip` | 16 MB | upstream product bundle; not part of the optimization evidence |
| `reports/v7-hardening/raw/profile/direct.trace` | 11 MB | summarized by `direct_*_top.txt` and `cold_cli_summary*.txt` |
| `reports/v7-hardening/raw/profile/semantic.trace` | 3.6 MB | summarized by `semantic_*_top.txt` and `cold_cli_summary*.txt` |

Small profile artifacts (`.pprof`, each < 80 KB) are **retained** as raw evidence; the
checksums in `reports/v7-hardening/raw/profile/SHA256SUMS.txt` for the removed `.trace`
files no longer resolve and are kept only as a historical record.

## What was preserved

- The MIT license and the upstream copyright notice (`Copyright (c) 2026 Asymptote Labs`).
- Upstream project documentation, product code, and attribution.
- `docs/optimization/` — the optimization case study.
- `examples/` — runnable memory examples.
- `reports/` — the benchmark and security evidence (identities redacted).
- `CHANGELOG.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, issue/PR templates.

## Provenance

This is a fresh, dependency-free copy: it shares no Git ancestry with the upstream
project or with any internal development history. See `README.md` for the optimization
summary and `docs/optimization/` for the full case study.
