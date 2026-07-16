# Phase 1: Walking skeleton and quality pipeline

## Implemented

- Added the Go control-plane module, Cobra composition root, daemon bootstrap,
  graceful shutdown, single-daemon PID lock, stale-lock recovery, and
  loopback-only address validation.
- Added private SQLite creation, embedded Goose migrations, sqlc queries, and a
  durable system-health schema version.
- Defined the versioned OpenAPI contract, generated Go server/client types and
  a TypeScript fetch client, and implemented `/api/v1/system` plus a typed
  WebSocket connection event.
- Added structured `slog` request logging and correlation IDs across the HTTP
  boundary.
- Built the Vue/Vite walking skeleton with live daemon, version, database, and
  event-stream state using TanStack Query.
- Added `switchyard version`, `daemon`, `ui`, and `doctor` commands and embedded
  the production Vue build in the Go binary.
- Added pinned toolchains, Make targets, generated-code checks, dependency
  automation, GitHub Actions jobs, linting, vulnerability scanning, unit/race,
  migration, browser, and visual-regression tests.
- Added an architecture analyzer plus a deliberate forbidden-import test and
  depguard policy.

## Files and modules added

- `cmd/switchyard` and `internal/bootstrap` process composition.
- `internal/foundation`, `internal/system`, and transport adapters under
  `internal/transport`.
- `internal/platform/sqlite`, `migrations`, `queries`, and generated sqlc code.
- `api/openapi.yaml`, Go contract generation, and generated TypeScript client.
- `web/src` walking-skeleton UI, Vitest coverage, Playwright E2E, and visual
  baseline.
- `tools/archcheck`, root toolchain configuration, Makefile, and CI workflows.

## Architecture decisions applied

- ADR-0001 dependency direction is enforced by `archcheck` and depguard.
- ADR-0002 keeps application and orchestration logic in Go.
- ADR-0003 provides one `switchyard` binary with daemon and client subcommands.
- ADR-0004 makes OpenAPI and the WebSocket envelope explicit contracts.
- ADR-0005 uses private local SQLite, sqlc, and forward-only embedded migrations.
- ADR-0013 restricts the Phase 1 browser server to loopback; authenticated local
  IPC, browser bootstrap sessions, and CSRF enforcement arrive in Phase 2.

## Tests added

- Go unit tests for build identity, correlation IDs, SQLite migration and file
  mode, system queries, CLI behavior, HTTP contracts, WebSocket envelopes,
  daemon addresses, lock exclusion, stale-lock recovery, and architecture rules.
- Go race suite and Windows cross-compilation of process-lock support.
- Vue component test with V8 coverage.
- Playwright end-to-end test against a real daemon, SQLite database, Vite proxy,
  and WebSocket stream.
- Full-page visual-regression baseline with a bounded cross-platform pixel
  tolerance.

## Commands run and results

```text
make generate: passed; OpenAPI, sqlc, and TypeScript outputs reproduced
make lint: passed; golangci-lint reported 0 issues, archcheck and ESLint passed
make typecheck: passed
make test: passed; Go packages and Vitest passed
make test-race: passed
make migrate-check: passed from an empty temporary directory
make vuln: passed; no vulnerabilities found
make test-e2e: passed; 1 browser test
make test-visual: passed; 1 visual test
make build: passed; production Vue assets embedded in bin/switchyard
GOOS=windows GOARCH=amd64 go test -c ./internal/bootstrap: passed
packaged smoke test: /api/v1/system, embedded UI, and switchyard doctor passed
permission smoke test: data directory 0700, lock and database 0600
shutdown smoke test: SIGINT and Playwright SIGTERM both removed daemon.lock
```

## Acceptance criteria status

- [x] `switchyard daemon` serves an embedded page showing real daemon status.
- [x] Local quality commands and matching CI jobs cover generation, formatting,
  lint, architecture, type, unit, race, migration, vulnerability, browser,
  visual, and release-build gates.
- [x] A deliberate forbidden domain-to-adapter import fails the architecture
  analyzer test.
- [x] SQLite creation and migration succeed from an empty data directory.
- [x] Graceful daemon and test-harness shutdown leave no lock-file corruption;
  valid stale PID locks also recover after abrupt termination.

## Known limitations

- Loopback HTTP is the Phase 1 browser transport, but privileged local IPC,
  browser bootstrap authentication, session cookies, and CSRF checks are Phase
  2 work.
- The event stream contains only the walking-skeleton connection event; durable
  operation replay begins in Phase 2.
- `switchyard ui` prints the local URL. Desktop-aware browser launching begins
  in Phase 8.

## Deferred work

- Phase 2 operation lifecycle, event journal, cancellation, recovery, local IPC,
  browser authentication, and CLI attach/start behavior.

## Manual verification

1. Run `make build`.
2. Run `./bin/switchyard daemon --data-dir .switchyard-data/manual`.
3. Open `http://127.0.0.1:19616` and confirm daemon, event stream, build, and
   database schema state are live.
4. Run `./bin/switchyard doctor` in another terminal.
5. Stop the daemon with Ctrl-C and confirm `daemon.lock` is absent.
