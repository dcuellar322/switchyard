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
switchyard ui [--path /projects/<id>]
switchyard plugin list
switchyard plugin refresh
switchyard doctor
```

Selection checks opaque ID first, then an exact unique slug, then a canonical
repository path. Missing and ambiguous selections fail instead of guessing.
Catalog removal never changes repository files. Trust and removal require
`--yes`; Switchyard does not hide an interactive confirmation inside automation
mode.

`ui` starts or attaches to the local daemon and prints a short-lived,
authenticated loopback URL. `--path` accepts only a local application route;
remote origins, fragments, dot segments, backslashes, and caller-supplied
bootstrap credentials are rejected. The native desktop adapter uses the same
command after its compatibility preflight.

`switchyard desktop snapshot --json` is a versioned, bounded native-adapter
contract containing daemon identity, project runtime/health summaries,
workspaces, recent operations, diagnostic alerts, host pressure, and
port-conflict count. It is
read-only except that normal CLI attachment may start the bundled daemon when
none is running. It is not intended as a replacement for the richer public
query commands.

`plugin list` and `plugin refresh` perform read-only package discovery. Trust
requires the exact displayed fingerprint and `--yes`; enable is a separate
decision requiring reviewed `--scope` values and `--yes`. `plugin disable`
revokes every grant. `plugin health`, `plugin logs`, and `plugin inspect` expose
bounded supervision and structured observations. `plugin run` accepts one
advertised typed action, a JSON object, and `--yes`, then queues a durable
audited operation. No plugin command exposes an arbitrary shell.

## Diagnosis and safe automation

```text
switchyard diagnose <project> [--provider <configured-provider>]
switchyard diagnose latest <project>
switchyard diagnose feedback <diagnosis> <hypothesis>
  --verdict accurate|false_positive [--note <local-note>]
switchyard diagnose run <diagnosis> <action> --yes
switchyard diagnose notifications [project] [--all]
switchyard diagnose notifications acknowledge <notification>

switchyard automation list [project]
switchyard automation create <project> <action>
  --name <name> --trigger <deterministic-code>
  [--cooldown 3600] [--max-per-day 3]
switchyard automation enable <recipe> --yes
switchyard automation disable <recipe>
switchyard automation evaluate <project>
```

Diagnosis always collects a bounded redacted bundle and runs deterministic
rules before an optional provider. Provider output can cite only evidence and
accepted non-destructive actions present in that bundle. `diagnose run` reloads
the durable result and queues only a cited accepted action through the normal
operation kernel; it cannot be used as an arbitrary action or shell launcher.

Feedback is stored locally and is never sent as telemetry. Notifications are
deduplicated local records for crashes, ports, resource pressure, and unhealthy
dependencies. Cleanup findings are previews only.

Automation recipes are created disabled. Enabling requires a separate `--yes`
review, and evaluation reacts only to deterministic trigger codes. Every recipe
has a cooldown and UTC daily limit, can be listed or disabled at any time, and
may run only read-only or declared test/check/inspect actions. Operation IDs are
returned for every dispatched action so outcomes remain inspectable and
audited.

## Runtime commands

Trusted Compose and native-process projects expose live runtime queries and
durable lifecycle operations:

```text
switchyard status <project>
switchyard plan <start|stop|restart|pause|unpause|rebuild|teardown> <project>
switchyard start|stop|restart|pause|unpause|rebuild <project>
switchyard teardown <project> [--volumes] --yes
switchyard logs <project> [--service name] [--since 10m] [--run id] [--operation id] [--tail 200]
switchyard logs <project> [--service name] [--run id] [--operation id] --export plain|ndjson
switchyard metrics <project> [--service name]
```

## Ports, Git, and trusted actions

```text
switchyard ports list
switchyard ports next [--range 15000-19999] [--protocol tcp|udp]
  [--project <project>] [--exclude 15001,15002]
switchyard git <project>
switchyard action list <project>
switchyard action run <project> <action> [--yes] [--allow-outside-root]
```

`ports list` combines accepted declarations, persistent stopped-project
reservations, attributed runtime bindings, and unmatched OS listeners. Every
row reports provenance, and conflict state is derived before startup. `ports
next` considers all of those facts plus explicit exclusions; `--project`
ignores that project's declarations and reservations, but never ignores a live
binding.

`git` is a fresh, read-only porcelain-v2 snapshot with branch or detached
state, change categories, ahead/behind, stashes, last commit, remotes,
merge/rebase state, and worktrees.

Actions are available only for trusted projects. Built-ins include terminal,
VS Code, Codex, Claude Code, and Git pull; accepted endpoint and manifest
actions add browser, test, migration, and project-specific commands. `action
run` returns a durable `action.run` operation immediately. Use `operation get`
to read its outcome. Destructive actions require `--yes`. Working directories
are confined to the trusted root after symlink resolution unless the caller
also supplies the explicit `--allow-outside-root` permission.

Lifecycle commands return a durable operation immediately. Read it with
`switchyard operation get <id>` or list recent project operations. `stop`
preserves containers and volumes. `teardown` is destructive, requires `--yes`,
and removes volumes only when the reviewed plan and operation include
`--volumes`.

Native process runtimes support `start`, `stop`, and `restart`. They reject
container-only pause, unpause, rebuild, teardown, and volume semantics. Start
plans show executable, argument array, working directory, environment-key
count, and keychain-reference count without revealing environment values.

`status` includes project ownership, service state, ports, and container or
process identity. Compose observations also include Docker context and Engine
versions. Process observations include verified PID/run fingerprints, restart
count, exit code, and last-known process identity. Logs support JSON Lines and
retain stdout/stderr plus project, service, source, and run identity. Metrics
return current CPU and memory samples; Docker also reports network counters.
Log reads come from the redacted persistent archive, not a daemon-lifetime
buffer. `--since` accepts a positive Go duration or RFC 3339 timestamp. Run and
operation filters retain lifecycle correlation in both queries and exports.

Daemon log bounds and extra redaction rules are configurable without changing
project manifests:

```text
switchyard daemon \
  --log-ring-entries 2000 \
  --log-segment-bytes 1048576 \
  --log-retention-age 168h \
  --log-retention-bytes 268435456 \
  --redact-pattern 'company-secret-[A-Za-z0-9]+'
```

## Workspaces and worktree environments

Worktree discovery is an explicit post-trust operation. Registration reads Git
administrative metadata, projects monorepo subdirectories into every checkout,
and persists stable environment identities and isolation allocations:

```text
switchyard environment list <project>
switchyard environment register <project>
switchyard environment get <environment-id>
switchyard environment hostname <environment-id> <name.localhost>
switchyard environment routes
```

Workspaces are reviewed JSON graph documents. Members may be trusted project
IDs or registered environment IDs. Dependencies must be acyclic; profiles can
select a dependency-closed subset and cap parallelism.

```text
switchyard workspace list
switchyard workspace get <workspace-id>
switchyard workspace create <definition.json>
switchyard workspace update <workspace-id> <update.json>
switchyard workspace delete <workspace-id> --yes
switchyard workspace start <workspace-id> [--policy rollback|continue]
  [--profile <profile-id>] [--run-recipes]
switchyard workspace stop <workspace-id> [--profile <profile-id>]
  [--remove-data --yes]
```

Start and stop return durable operations immediately. Start follows dependency
order while running independent branches in parallel; stop reverses that
order. Rollback and continue policies report per-member outcomes. Stop
preserves data by default. `--remove-data` is destructive and is rejected
without `--yes`.

Friendly routing is disabled unless the daemon is started with a loopback-only
`--routing-address`. It accepts HTTP targets only, reports unavailable and
conflicting routes explicitly, and returns `503` for unknown hostnames. Local
TLS is not implied by this option.

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
## Offline data and manifest migration

`switchyard data inspect`, `data backup`, and `data migrate` operate directly
on the configured data directory without starting the daemon. Inspection and
migration preview are read-only. `data migrate --write` requires the daemon to
be stopped and creates a verified non-overwriting pre-migration backup before
changing schema. `data backup --output <path>` refuses to overwrite its output.

`switchyard manifest migrate <path>` previews the stable v1 YAML. `--write`
creates a `.v1alpha1.bak` copy and atomically replaces the source. These
commands follow the same JSON envelope and semantic error behavior as other
CLI groups.
