# Phase 8: Ports, Git, and actions

## Implemented

- Added a declared/reserved/bound port domain with accepted-manifest and
  Compose provenance, SQLite-backed stopped-project reservations, attributed
  runtime bindings, deduplicated `lsof` TCP/UDP listeners, stable conflict IDs,
  wildcard-host reasoning, partial-source warnings, and preferred-range
  suggestions with exclusions.
- Added fresh read-only Git observation using porcelain v2, including branch or
  detached state, staged/modified/untracked/conflicted counts, ahead/behind,
  stash count, last commit, remotes, worktrees, and merge/rebase state.
- Added typed and risk-classified manifest actions plus built-in terminal, VS
  Code, Codex, Claude Code, Git pull, and accepted endpoint actions.
- Added durable `action.run` operations with destructive confirmation,
  cancellation and timeouts, symlink-aware root containment, explicit
  outside-root permission, a narrow environment allowlist, bounded optional
  output capture, explicit shell opt-in, and privilege-escalation rejection.
- Added macOS terminal, editor, agent, and HTTP/HTTPS browser launch adapters.
  Terminal and agent tests assert the exact resolved working directory.
- Added redaction-safe action audit persistence and daemon-restart recovery.
- Added generated OpenAPI Go and TypeScript clients, private IPC clients, CLI
  commands, a live port registry, and project Git/quick-action UI slices.
- Added one deterministic port-conflict visual reference and extended real
  browser onboarding coverage through Git, actions, and port evidence.
- Added durable preferred port ranges and exclusions used by browser
  suggestions, plus durable terminal/editor preferences that filter built-in
  quick actions without changing explicitly accepted manifest actions.

## Files and modules added

- `internal/ports/{domain,application,adapters}`
- `internal/sourcecontrol/{domain,application,adapters}`
- `internal/actions/{domain,application,adapters}`
- `internal/platform/sqlite/ports.go`
- `internal/platform/sqlite/action_audit.go`
- `internal/transport/cli/developer_commands.go`
- `internal/transport/httpapi/developer_handler.go`
- `migrations/00006_ports_actions.sql`
- `web/src/domains/ports`
- `web/src/domains/projects/components/ProjectDeveloperTools.vue`
- `docs/architecture/developer-workflows.md`

## Architecture decisions

No new ADR was required. The implementation realizes the existing modular
monolith, manifest precedence, SQLite, local transport security, log/audit
redaction, and platform-order decisions. Port, source-control, and action policy
remain separate application boundaries. macOS was the first native launcher;
Phase 18 subsequently added the Linux and Windows adapters required by
ADR-0015.

Action commands are never built by UI or transport code. The generated API
submits a durable operation, and the operation executor invokes the trusted
action service. This retains one cancellation, serialization, recovery, and
event model rather than adding an action-specific job system.

## Tests added

- Port declaration/reservation/binding reconciliation, stopped-vs-running
  conflict visibility, same-project service conflicts, logical-claim
  de-duplication, wildcard hosts, unknown listeners, listener-row
  de-duplication, and suggestion exclusions.
- Real temporary Git repository updates after a file change plus porcelain
  branch/change/commit/worktree parsing.
- Manifest action invariants, destructive confirmation, parent and symlink
  escape rejection, explicit escape permission, exact terminal working
  directory, command cancellation, environment allowlisting, and `sudo`/`doas`
  rejection.
- SQLite reservation cleanup, audit outcome persistence, and schema migration
  6.
- HTTP empty-array contracts and confirmed/unconfirmed action operation paths.
- Vue conflict, Git, action, loading/error, and free-port suggestion states.
- Playwright real-daemon and deterministic visual coverage.

## Verification evidence

```text
go test ./internal/actions/... ./internal/manifest/...
PASS

go test ./internal/ports/...
PASS

make lint
PASS: gofmt, go vet, golangci-lint, architecture check, frontend ESLint

pnpm --dir web typecheck
pnpm --dir web test
PASS

pnpm --dir web test:e2e
PASS: 2 real-daemon browser flows

pnpm --dir web test:visual
PASS: system and deterministic port-conflict baselines

make build
PASS: production Vue bundle and packaged Go binary

make quality
PASS: generated-code drift, lint, typechecking, unit and race tests, fresh
database migration, vulnerability scan, isolated browser E2E, visual baselines,
and production build
```

Packaged acceptance used a fresh temporary schema-6 database and private Unix
socket. The CLI trusted the action fixture, returned live Git branch/change and
worktree state, listed the built-in and accepted actions, suggested port
`15000/tcp` after reading real host listeners, queued `verify`, and observed
its durable operation reach `succeeded`. The audit row stored `ipc`,
`read_only`, `command`, and the exact fixture root, with no output or
environment values.

## Acceptance criteria status

- [x] A stopped project's persistent reservation conflicts with another
  project's live binding before startup and is rendered with both sources.
- [x] Port evidence identifies accepted manifest, Compose-derived, runtime, and
  live OS-listener sources without collapsing their fact kinds.
- [x] Git state is read fresh and updates after file or repository changes.
- [x] Manifest approval and execution-time symlink resolution both prevent
  implicit escape from the trusted project root; execution requires an
  explicit permission to cross it.
- [x] The macOS terminal adapter is tested to launch at the exact resolved
  project working directory.

## Known limitations and deferred work

- Automatic repository rewriting for port remediation is intentionally out of
  scope. The registry explains conflicts and suggests a free port only.
- Git commit, merge, rebase, and stash mutation UI is intentionally deferred.
- Action output is bounded for process safety but is not yet a first-class log
  stream; persistent process and Compose logs continue to use the Phase 7
  observability pipeline.
