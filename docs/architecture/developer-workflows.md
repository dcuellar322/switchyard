---
title: Ports, source control, and trusted actions
description: Project port evidence, Git observation, and risk-classified developer actions.
category: concept
audience: [user, contributor, integrator]
since: 1.0.0
lastVerified: 2026-07-17
---

Phase 8 adds three independent application boundaries for common local
development work. They share catalog identity and the durable operation kernel,
but they do not read each other's storage or move policy into transport code.

## Port evidence and reservations

The port registry keeps three kinds of fact distinct:

- A `declaration` comes from an accepted effective manifest. Compose-derived
  declarations retain `compose` provenance; native declarations retain
  `manifest` provenance.
- A `reservation` is a persistent Switchyard lease reconciled atomically from
  accepted declarations. It protects stopped projects and is removed when the
  accepted manifest no longer declares the port.
- A `binding` is current runtime or operating-system evidence. Runtime bindings
  are attributed to a trusted project; unmatched `lsof` TCP/UDP listeners are
  intentionally labeled as unknown processes.

Each fact carries its source, evidence text, host, protocol, project/service
identity when known, and observation time. Runtime and OS observation failures
produce explicit partial-data warnings. They do not make stale or invented
facts look current.

Conflict classification understands wildcard host overlap and logical claims.
A declaration, its reservation, and its own runtime binding are one claim;
different services in one project can still conflict. Repeated socket rows and
multiple unknown worker processes sharing one listener are not multiplied into
false conflicts. The public classifications are:

```text
DECLARED_VS_DECLARED
DECLARED_VS_RESERVED
DECLARED_VS_BOUND
RESERVED_VS_RESERVED
BOUND_BY_UNKNOWN_PROCESS
PROTOCOL_MISMATCH
HOST_ADDRESS_OVERLAP
```

Free-port suggestions search an explicit range and consider declarations,
reservations, runtime bindings, OS listeners, caller exclusions, and an
optional project whose non-binding facts may be ignored. Suggestions never
rewrite repository files.

## Read-only Git observation

The source-control module receives only a trusted canonical project root. Its
adapter invokes the installed Git CLI with stable formats:

- `status --porcelain=v2 --branch --show-stash` for branch, detached state,
  upstream divergence, stash count, and staged/modified/untracked/conflicted
  counts;
- a bounded one-commit format for last-commit identity;
- `remote -v` and `worktree list --porcelain` for remotes and worktrees; and
- `rev-parse --git-path` plus filesystem existence checks for merge/rebase
  state.

Every query is fresh, so file and repository changes are visible without a
daemon restart. A trusted non-Git directory returns `repository: false` rather
than failing the project. Observation does not run project commands or mutate
the repository.

## Trusted action execution

Actions are accepted manifest definitions or narrow built-in conveniences.
Built-ins open a terminal, VS Code, Codex, Claude Code, perform `git pull`, and
open accepted endpoint URLs. Manifest-defined test, migration, and command
actions remain explicit argument arrays. A manifest definition with a built-in
ID overrides that default visibly.

Every action has a type and one of `read_only`, `mutating`, `networked`,
`destructive`, or `interactive`. Destructive actions require an explicit
confirmation bit before an operation can be queued. Execution then follows one
path:

```text
HTTP or IPC request
  -> trusted action lookup and risk check
  -> durable action.run operation
  -> symlink-aware working-directory resolution
  -> audit start
  -> typed platform adapter or bounded command runner
  -> audit and operation terminal outcome
```

The working directory is resolved again at execution time. Parent paths,
absolute paths, and symlink changes cannot escape the trusted project root
unless the caller supplies the explicit `allowOutsideRoot` permission. The
manifest approval validator independently rejects escaping paths, providing a
second boundary before an action becomes trusted.

Commands use argument arrays by default, a timeout and cancellable context, a
small ambient environment allowlist plus reviewed manifest overlay, and an
optional one-megabyte in-memory output cap. Output and environment values are
never written to the action audit. Shell execution requires `shell: true` and
exactly one command string. Both array and shell forms reject `sudo` and
`doas`; Switchyard never elevates privileges for an action.

On macOS, terminal and editor launch use native `open` behavior. Agent launch
uses a quoted AppleScript command whose first operation changes to the exact
resolved working directory. Browser actions accept only HTTP and HTTPS URLs.
Unsupported platforms return an honest capability error until their later
platform phase.

## Persistence and recovery

Migration 6 stores manifest reservations and redaction-safe action audits.
Audit rows contain operation, project, action, type, risk, actor, resolved
working directory, timestamps, terminal state, and a stable error code. They do
not contain command output, arguments, or environment values. A daemon restart
marks an audit left in `running` as failed with `DAEMON_RESTARTED`, while the
ordinary operation recovery path handles the corresponding durable operation.

The generated OpenAPI clients, CLI, and Vue slices all use these application
boundaries. No UI component builds a Git, `lsof`, terminal, editor, or action
command.
