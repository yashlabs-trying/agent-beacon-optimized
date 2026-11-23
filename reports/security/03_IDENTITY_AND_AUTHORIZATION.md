# 03 — Identity and Authorization (Security Layer 1)

Status: **implemented and tested** for the mechanism. Deployment identity
guarantee: **BLOCKED** (no auth provider), documented below.

## Objective

Prevent unauthorized memory reads, writes, searches, and trust transitions
between agents or projects sharing a service. Authority must come from a
server-side decision, never from MCP request arguments.

## What changed

| File | Change |
|---|---|
| `cli/beacon/internal/mcpserver/server.go` | Added `Authorizer` interface, `AllowAll` (local default), `ProjectAllowlist` (fail-closed), `ErrProjectNotAuthorized`; `Options.Authorizer`; a single `authorizeProject` choke point; `authorizedMemories` defense-in-depth filter. `get_memory` now authorizes against the memory's own project. Search/context resolve the project (including a relayed path) and authorize before querying. |
| `cli/beacon/internal/mcpserver/server.go` | `memoryStore()` now opens **one** shared store per server (was: a fresh, never-closed `*sql.DB` pool per call). Added `Server.Close()`. |
| `cli/beacon/cmd/mcp.go` | Added `--project-allowlist` (and `BEACON_MCP_PROJECT_ALLOWLIST`); HTTP transport warns when no allowlist is set. |

## Authorization model

1. **Caller** — in the declared deployment, the OS process/user that owns the
   store. stdio inherits the parent harness's identity; loopback HTTP is the
   same OS user.
2. **Permitted projects** — configured by the operator via the allowlist. Empty
   or absent means the documented single-user local mode (`AllowAll`).
3. **Operation** — read/search/context/ID-lookup.
4. **Target project** — resolved server-side (from the store, not the request)
   for `get_memory`; from the request but **checked** for search/context.
5. **Decision** — `Authorizer.AuthorizeProject(ctx, projectID)`.

Rules:
- An empty/nil `ProjectAllowlist` **denies everything** (fail-closed).
- `get_memory` authorizes the fetched memory's project; a denial returns the
  same "memory not found" error as a missing ID, so existence is not disclosed.
- Search/context authorize the resolved project before touching the store, and
  drop any returned row whose project is not authorized (defense in depth).
- A request-supplied `project` path is resolved by `Store.ProjectIDForPath`
  (no filesystem access) and then authorized; an unknown path maps to an ID that
  matches no rows.

## Tests

`cli/beacon/internal/mcpserver/security_authorization_test.go` (all PASS):

| Test | Asserts |
|---|---|
| `TestAuthorizationGetMemoryCrossProjectDenied` | unlisted project's memory is denied and its body is not leaked |
| `TestAuthorizationGetMemoryAllowedProjectSucceeds` | authorized access still works |
| `TestAuthorizationSearchRejectsUnlistedProject` | caller `project_id` substitution denied |
| `TestAuthorizationContextRejectsUnlistedProject` | context tool denies substitution |
| `TestAuthorizationSearchAllowedProjectSucceeds` | authorized search works |
| `TestAuthorizationEmptyAllowlistDeniesEverything` | fail-closed |
| `TestAuthorizationUnknownProjectIDDenied` | unknown project denied |
| `TestAuthorizationDefaultModeUnchanged` | local default not regressed |
| `TestProjectAllowlistFailClosed` | authorizer contract (nil/empty/trim) |

`cli/beacon/internal/mcpserver/security_baseline_repro_test.go` documents the
pre-fix default behavior (still passes, because it runs without an allowlist —
the documented local mode).

Command:

```
go test ./internal/mcpserver/ -count=1
ok  github.com/asymptote-labs/agent-beacon/cli/beacon/internal/mcpserver
```

## Performance overhead

Authorization is an O(1) map lookup per project decision, plus one lookup per
returned row in `authorizedMemories`. In the default local mode `AllowAll` is a
no-op. No measurable retrieval-path cost is claimed without measurement; the
per-row check is a map membership test on already-materialized results.

## Acceptance

| Requirement | Status |
|---|---|
| Project-scoped get_memory | PASS |
| Project-scoped search/context | PASS |
| Server-resolved project permissions | PASS |
| Denial of unauthorized project switching | PASS |
| No disclosure via existence/errors | PASS (not-found for get_memory) |
| Fail-closed default for allowlist | PASS |
| Local single-user mode preserved | PASS |
| Authenticated caller identity for shared HTTP | **BLOCKED** (no auth provider) |

## Limitations / honest scope

- In the default (`AllowAll`) mode there is **no** cross-project enforcement:
  that is correct for the declared single-user local deployment (the caller can
  read `memory.db` directly anyway) and is **not** claimed as protected.
- There is no cryptographic caller identity. A shared HTTP endpoint relies on
  loopback binding plus the OS user boundary; if that is insufficient for a
  deployment, the operator must set `--project-allowlist`. Authenticated MCP
  caller identity remains BLOCKED.
- The authorizer is per-project, not per-operation: write/approval authorization
  is enforced at the CLI/reviewer boundary, not yet at an MCP mutation tool
  (there is no MCP mutation tool). This is reported as a remaining gate.
