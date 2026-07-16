# MCP reference

Switchyard's MCP server makes local project operations available to Codex,
Claude Code, and other MCP clients without asking them to guess Docker or shell
commands. It uses stdio and connects to the ordinary Switchyard daemon over its
privileged local IPC endpoint.

## Install

Build or install the `switchyard` binary, then from a repository root run:

```bash
switchyard agent install codex --profile develop
switchyard agent install claude --profile develop
```

Run only the installer matching the client you use. `observe` is the safer
default and has no mutation tools. Use `--project PROJECT_ID` one or more times
to constrain reads and mutations to explicit registered projects. A restricted
server cannot scan/register a new project, and proposal acceptance resolves the
proposal's owner before authorization. Inspect all options with
`switchyard agent install --help`.

To configure another MCP client directly:

```json
{
  "type": "stdio",
  "command": "/absolute/path/to/switchyard",
  "args": [
    "--data-dir", "/absolute/path/to/data",
    "mcp", "serve",
    "--transport", "stdio",
    "--provider", "generic",
    "--agent-id", "my-client",
    "--profile", "observe"
  ]
}
```

MCP owns stdout while running. Diagnostics go to stderr.

## Tools

All tool names are stable and prefixed with `switchyard_`. Inputs reject
unknown fields through SDK-generated JSON Schemas. Identifiers are opaque;
obtain project, service, action, proposal, and operation IDs from read tools.

### Observe

| Tool | Key input | Bound or behavior |
|---|---|---|
| `switchyard_system_info` | none | Daemon/API/schema readiness |
| `switchyard_projects_list` | `limit` | Default 50, maximum 100 |
| `switchyard_project_get` | `projectId` | One catalog project |
| `switchyard_project_status` | `projectId` | Catalog + runtime + health |
| `switchyard_project_services` | `projectId` | Maximum 100 services |
| `switchyard_project_logs_query` | filters, `tail` | Redacted; default 100, maximum 500 |
| `switchyard_project_health` | `projectId` | Current persisted health |
| `switchyard_project_health_wait` | `projectId`, `timeoutSeconds` | Wait for healthy; maximum 30 seconds |
| `switchyard_project_git_status` | `projectId` | Bounded structured Git facts |
| `switchyard_ports_list` | optional `projectId`, `limit` | Maximum 500 facts and 100 conflicts |
| `switchyard_ports_suggest` | range, protocol, `requestId` | At most 10,001 candidates; no reservation |
| `switchyard_actions_list` | `projectId` | Maximum 100 accepted actions |
| `switchyard_operation_get` | `operationId` | Durable current state |
| `switchyard_operation_wait` | `operationId`, `timeoutSeconds` | Terminal-state wait; maximum 30 seconds |
| `switchyard_manifest_explain` | `projectId` | Effective manifest and provenance |

### Develop and above

`switchyard_project_start`, `switchyard_project_stop`,
`switchyard_project_restart`, `switchyard_project_pause`, and
`switchyard_project_resume` accept:

```json
{
  "projectId": "project-opaque-id",
  "serviceIds": ["api"],
  "requestId": "stable-request-123"
}
```

Omit `serviceIds` for the whole project. A targeted start includes required
native-process dependencies. Targeted Compose operations use only declared
service names. Teardown cannot be targeted.

`switchyard_action_run` accepts `projectId`, `actionId`, `requestId`, and the
explicit `confirmRisk` and `allowOutsideRoot` gates. The develop profile cannot
run a destructive action even when confirmation is supplied.

`switchyard_operation_cancel` accepts an operation ID and request ID. It asks
the durable coordinator to cancel and returns the updated operation.

### Maintain and admin

Maintain adds `switchyard_project_rebuild` and
`switchyard_manifest_proposal_create`. Proposal creation performs deterministic
safe reads and does not execute repository code.

Admin adds `switchyard_manifest_proposal_accept` and
`switchyard_project_teardown`. Teardown accepts `removeVolumes`; the boolean
must be explicit and the tool is annotated destructive. Accepting a proposal
is an explicit trust transition and never happens during proposal creation.

## Resources and prompts

Static resources are `switchyard://system` and `switchyard://projects`.
Project templates provide catalog, status, manifest, services, Git, ports, and
at most 50 recent redacted errors under
`switchyard://projects/{projectId}/...`.

The server publishes four prompts:

- `switchyard_onboard_project`
- `switchyard_diagnose_project`
- `switchyard_start_and_verify`
- `switchyard_resolve_port_conflict`

Prompts describe safe tool workflows; they grant no extra capability.

## Typical lifecycle

1. Call `switchyard_projects_list` and select an opaque project ID.
2. Call `switchyard_project_status`.
3. Call `switchyard_project_start` with a new stable request ID.
4. Call `switchyard_operation_wait` with the returned operation ID.
5. Call `switchyard_project_health_wait`.
6. If unhealthy, read `switchyard_project_health` and bounded redacted logs.
7. To restart one declared service, call `switchyard_project_restart` with its
   service ID, wait for the operation, and verify health again.

Never retry a different mutation with the same request ID.

## Inspector

The repository pins the smoke-test version and runs it with:

```bash
make test-mcp-inspector
```

The test builds the production binary, starts an isolated daemon, invokes the
MCP Inspector CLI over stdio, and validates discovery plus default destructive
tool omission.
