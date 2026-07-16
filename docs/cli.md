# Switchyard CLI contract

The `switchyard` binary is both the daemon and its terminal client. Client
commands use private local IPC and start the same binary as a detached daemon
when no healthy daemon is already listening. The command result does not change
based on which process started the daemon.

## Project and operation commands

```text
switchyard list
switchyard add <repository>
switchyard project list
switchyard project get <id|unique-slug|path>
switchyard project add <repository>
switchyard project trust <project> --yes
switchyard project remove <project> --yes

switchyard operation list [--project <project>] [--limit 100]
switchyard operation get <operation-id>
switchyard operation cancel <operation-id>

switchyard manifest explain <project>
switchyard manifest diff <project>
switchyard manifest validate <project>
switchyard open <project> [--print]
switchyard doctor
```

Selection checks opaque ID first, then an exact unique slug, then a canonical
repository path. Missing and ambiguous selections fail instead of guessing.
Catalog removal never changes repository files. Trust and removal require
`--yes`; Switchyard does not hide an interactive confirmation inside automation
mode.

## Runtime commands

Trusted Compose projects expose live runtime queries and durable lifecycle
operations:

```text
switchyard status <project>
switchyard plan <start|stop|restart|pause|unpause|rebuild|teardown> <project>
switchyard start|stop|restart|pause|unpause|rebuild <project>
switchyard teardown <project> [--volumes] --yes
switchyard logs <project> [--service name] [--since 10m] [--tail 200]
switchyard metrics <project> [--service name]
```

Lifecycle commands return a durable operation immediately. Read it with
`switchyard operation get <id>` or list recent project operations. `stop`
preserves containers and volumes. `teardown` is destructive, requires `--yes`,
and removes volumes only when the reviewed plan and operation include
`--volumes`.

`status` includes the resolved Docker context, negotiated Engine versions,
project ownership, health, container metadata, and published ports. Logs support
JSON Lines and retain stdout/stderr plus project, service, and container-run
identity. Metrics return current CPU, memory, and network samples.

## Automation modes

All query and mutation commands accept global flags after or before the command:

```text
--json             one indented versioned envelope
--jsonl            one compact envelope per list/stream item
--non-interactive  guarantee that no prompt is attempted
--no-color         suppress ANSI output
```

Switchyard currently emits no ANSI sequences when output is redirected, and
all Phase 4 commands are prompt-free. JSON has this top-level shape:

```json
{
  "schemaVersion": "switchyard.cli/v1",
  "command": "project.get",
  "data": {}
}
```

Errors are written to stderr as one object when `--json` or `--jsonl` is set:

```json
{
  "schemaVersion": "switchyard.cli/v1",
  "error": {
    "code": "PROJECT_NOT_FOUND",
    "message": "project ... was not found by ID, slug, or path",
    "exitCode": 3
  }
}
```

`switchyard schema cli <command>` prints the current draft 2020-12 envelope
schema and its generated OpenAPI data-model reference. Use `switchyard
completion bash|zsh|fish|powershell` to generate shell completion.

## Exit codes

| Code | Meaning |
|---:|---|
| 0 | Success |
| 2 | CLI usage, flags, confirmation, or output-mode error |
| 3 | Requested project or operation was not found |
| 4 | Ambiguous selection, state conflict, or invalid proposal |
| 5 | Local daemon unavailable or failed to start |
| 6 | Internal or unclassified API failure |

Command packages only resolve input and render output. Catalog, manifest, and
operation behavior remains in application services behind the generated local
API.
