# Phase 2: Operations kernel and local IPC

## Implemented

- Added the durable operations domain and validated queued, running, succeeded,
  failed, cancelled, and partially-succeeded transitions.
- Added idempotent operation creation, append-only steps, mutation audit events,
  compare-and-swap state updates, durable cancellation, same-project
  serialization, and cross-project concurrency.
- Defined restart behavior: queued work resumes, queued cancellation completes,
  interrupted running work fails with `DAEMON_RESTARTED`, and terminal work is
  unchanged.
- Added a persistent, monotonic SQLite event journal with bounded replay,
  live fan-out, slow-subscriber disconnect, and client reconnect from the last
  sequence.
- Added private HTTP-over-Unix-socket transport for macOS/Linux and a Windows
  named-pipe address abstraction for the Phase 18 adapter.
- Split privileged IPC and loopback browser routers while sharing generated
  OpenAPI handlers and application services.
- Added one-time browser bootstrap tokens, HttpOnly SameSite sessions, CSRF
  enforcement, idempotency-key validation, exact WebSocket origin validation,
  and browser security headers.
- Updated `switchyard doctor` and `switchyard ui` to use local IPC. The latter
  returns a one-time authenticated browser URL.
- Added stable RFC 9457-style error codes and generated operation/session API
  contracts for Go and TypeScript.

## Files and modules added

- `internal/operations/domain` and `internal/operations/application`.
- `internal/foundation/events` and `internal/foundation/identifier`.
- `internal/platform/localipc`, SQLite operation repository, and SQLite event
  journal.
- `internal/session/application`.
- HTTP security, session, and operation handlers plus daemon server-group
  lifecycle composition.
- Migration `00002_operations_kernel.sql` and generated sqlc queries.
- Browser session bootstrap helper, reconnecting event composable, and browser
  test bootstrap helper.
- `docs/architecture/operations-kernel.md`.

## Architecture decisions

- ADR-0001 is preserved: operation invariants have no transport or persistence
  imports; SQLite and local IPC are adapters.
- ADR-0003 is applied by using the same binary for daemon, IPC clients, and
  browser bootstrap.
- ADR-0004 is applied through generated OpenAPI types and a persistent,
  versioned WebSocket envelope.
- ADR-0005 is applied with migration 2, typed sqlc queries, WAL storage,
  compare-and-swap transitions, steps, audit, and journal tables.
- ADR-0013 is applied with private Unix IPC plus authenticated, CSRF-protected,
  same-origin loopback browser access.

## Tests added

- Operation state-machine transition tests.
- Idempotency, same-project serialization, cross-project concurrency,
  cancellation, progress, and restart-recovery tests against real SQLite.
- Event journal persistence, replay, and live publication tests.
- Unix socket exclusivity, HTTP transport, and `0600` permission tests.
- One-time browser credential, expiration, CSRF, unauthenticated query,
  same-origin WebSocket, and authenticated mutation tests.
- WebSocket replay-after-sequence tests and frontend bootstrap unit coverage.
- Playwright browser launch now exercises CLI -> Unix IPC -> bootstrap token ->
  loopback cookie session -> API/WebSocket.

## Commands run and results

```text
make generate: passed; Go, sqlc, and TypeScript outputs reproduced
make lint: passed; golangci-lint 0 issues, archcheck and ESLint passed
make typecheck: passed
make test: passed; all Go packages and 2 Vitest files passed
make test-race: passed
make migrate-check: passed; schema version 2 from empty private storage
make test-e2e: passed; authenticated IPC-to-browser path
make test-visual: passed
make build: passed; authenticated Vue client embedded
make vuln: passed; no vulnerabilities found
Windows and Linux bootstrap cross-compilation: passed
packaged doctor over Unix IPC: ready, API v1, schema 2
packaged browser smoke: unauthenticated 401, token exchange 201, token reuse 401,
  missing CSRF 403, authorized missing operation 404
permission smoke: data directory 0700; database and Unix socket 0600
shutdown smoke: daemon.lock and switchyard.sock both removed
```

## Acceptance criteria status

- [x] Duplicate idempotent submissions return one operation and execute once.
- [x] Two mutations for one project never execute concurrently.
- [x] Mutations for independent projects execute concurrently.
- [x] CLI system and bootstrap traffic uses a private Unix socket without the
  loopback TCP API.
- [x] Browser mutations fail without a valid session, CSRF token, and
  idempotency key.

## Known limitations

- No production project lifecycle executor exists yet; Phase 2 tests use an
  explicit fixture executor, and the daemon fails unknown recovered kinds
  rather than guessing behavior.
- Windows reserves the named-pipe identity but uses no privileged pipe listener
  until the Phase 18 Windows adapter is implemented.
- Browser sessions are intentionally in memory and are invalidated by daemon
  restart.

## Deferred work

- Phase 3 project catalog, trust approval, manifests, and deterministic
  discovery producers.
- Phase 4 operation CLI list/get/cancel and consistent machine output.
- Runtime-specific reconciliation of interrupted external side effects begins
  with the Compose and native-process phases.

## Manual verification

1. Run `make build` and start the daemon with a temporary `--data-dir`.
2. Run `switchyard --data-dir <same-dir> doctor`; verify it succeeds through
   `switchyard.sock`.
3. Run `switchyard --data-dir <same-dir> ui` and open its one-time URL.
4. Confirm reusing the bootstrap URL fails and direct `/api/v1/system` requests
   without its cookie return `SESSION_REQUIRED`.
5. Stop the daemon and confirm both `daemon.lock` and `switchyard.sock` are gone.
