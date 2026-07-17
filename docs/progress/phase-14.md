---
title: "Phase 14: Embedded terminals and agent sessions"
description: Implementation evidence for Switchyard product phase 14.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-17
---

## Implemented

- Added a provider-neutral terminal domain for typed shell, Compose service,
  database-client, coding-agent, and reviewed interactive-action sessions.
- Added a daemon-owned Unix PTY adapter with explicit working directory,
  argument-array commands, terminal environment, resize, and process-group
  termination. Phase 18 subsequently added the native ConPTY adapter.
- Added owner-scoped application lifecycle, bounded 1 MiB reconnect scrollback,
  non-blocking subscribers, a 30-minute detached idle deadline, explicit
  termination, daemon-shutdown interruption, and restart recovery.
- Added SQLite schema version 11 for metadata-only session and audit records.
  Terminal bytes, input, commands, arguments, and environment values are not
  persisted.
- Added generated REST and TypeScript contracts for terminal and agent session
  create/list/get/terminate operations.
- Added authenticated same-origin terminal and agent WebSockets. Binary frames
  carry PTY bytes; bounded JSON frames carry resize control; unknown controls,
  oversized input, owner mismatch, and slow consumers are rejected.
- Added typed resolution through accepted catalog, action, environment, and
  Compose facts. Custom commands require an accepted `interactive` action;
  privilege-escalating shell text and path-escaping Compose inputs are denied.
- Added xterm.js with Fit and WebLinks addons, screen-reader mode, debounced
  resize, bounded presentation scrollback, modifier-gated HTTP(S) links, full
  session-state UI, reconnect controls, and explicit process termination.
- Added project Terminal and Agents surfaces. Agent metadata includes provider,
  project, worktree, cwd, status, output byte count, and literal
  `user_visible_terminal_output_only` capture semantics.
- Preserved trusted external-terminal handoff beside the embedded terminal.
  MCP deliberately retains no generic terminal, shell, SQL, or Docker tool.

## Architecture decisions

No ADR changed. The implementation applies ADR-0004's REST/WebSocket split,
ADR-0005's domain-owned SQLite boundary, ADR-0007's process ownership,
ADR-0010 and ADR-0011's provider-neutral least-privilege agent integration,
ADR-0013's local transport security, and ADR-0015's Unix-first platform order.

The persistence policy is an implementation choice within those accepted
decisions: a browser disconnect detaches, the daemon keeps the PTY for a
bounded idle window, and restart interrupts rather than fabricating resume.

## Safety properties

- Public APIs cannot submit a raw command. The resolver emits an argument array
  only after accepted project trust and selected-worktree validation.
- Session ownership is checked below the transport for list, get, attach, and
  terminate. Browser authentication and exact same-origin validation happen
  before the WebSocket reaches that boundary.
- A slow subscriber is detached without blocking PTY output. Client input,
  terminal dimensions, subscriber queues, reconnect output, and browser
  scrollback all have explicit bounds.
- Audits contain lifecycle events, principals, dimensions, byte counts, and
  process identity only. They contain no terminal input/output or environment.
- Terminal links accept only HTTP(S), require a modifier key, and use
  `noopener,noreferrer`.

## Tests added

- Typed launch validation, owner identity validation, start failure, request
  cancellation, detach persistence, reconnect scrollback, explicit termination,
  idle expiry, restart interruption, resize/input forwarding, and slow-subscriber
  fanout load.
- Compose declaration/path resolution, custom-action risk, privilege escalation,
  working-directory containment, argument arrays, and environment forwarding.
- Real Unix PTY Unicode, ANSI color, alternate-screen sequences, resize, exit,
  and process-group termination.
- SQLite metadata/audit round trips, missing-session mapping, restart recovery,
  schema version 11, and absence of sensitive payload columns.
- REST translation and filtering, browser cookie/origin protection on both PTY
  paths, WebSocket ready/snapshot/input/resize/output/exit protocol, and invalid
  control rejection.
- Vue typed launcher and agent disclosure unit tests, live browser shell and
  reconnect E2E coverage, and terminal visual regression.

## Acceptance criteria status

- [x] A real interactive shell opens at the trusted project or selected
  worktree directory.
- [x] Resize, Unicode, ANSI color, and alternate-screen/full-screen control
  sequences work through a real Unix PTY.
- [x] Browser close behavior is documented and implemented as bounded detach
  with reconnect, explicit termination, and idle expiry.
- [x] Missing/invalid browser sessions, cross-origin requests, and owner
  mismatches cannot attach to PTYs.
- [x] Agent records use a literal user-visible-output capture policy and never
  claim hidden-reasoning access.

## Verification evidence

Verified on 2026-07-16 with `make quality`:

- generated OpenAPI, TypeScript, JSON Schema, and SQL code had no drift;
- `gofmt`, `go vet`, golangci-lint, architecture checks, ESLint, and TypeScript
  checks passed;
- all Go tests, 20 Vue unit tests, the Go race suite, and a clean schema-11
  migration passed;
- `govulncheck ./...` reported no vulnerabilities;
- four Chromium E2E scenarios passed against the live daemon, including a
  real `sh` PTY in the mixed-project fixture, UTF-8 output, browser detach and
  reconnect, explicit termination, native-process lifecycle, and Compose
  lifecycle;
- nine Chromium visual regressions passed, including the typed terminal
  launcher and persistence disclosure; and
- the production Vue bundle and trimmed Go binary built successfully.

The Unix adapter integration suite separately exercised ANSI color,
alternate-screen sequences, resize propagation, Unicode, normal exit, and
process-group termination through a real PTY.

## Known limitations and deferred work

- Unix PTYs are production-capable on macOS/Linux. Native Windows ConPTY is now
  supplied by Phase 18 rather than a silent pipe fallback.
- Rich provider SDK orchestration is deferred by the scope guard. Codex and
  Claude Code run through their reliable user-visible PTY interfaces.
- PTY scrollback is intentionally not durable across daemon restarts.
