---
title: "Phase 1: Walking skeleton and quality pipeline"
description: Implementation evidence for Switchyard product phase 1.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-17
---

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

Browser E2E and visual tests use a dedicated, overridable daemon address so the
quality gate can run while a normal Switchyard daemon is active on port 19616.

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

## 2026-07-17 CI stabilization evidence

- pnpm 11's build-script allowlist now explicitly approves the pinned
  `esbuild` and `vue-demi` packages, so frozen installs no longer stop before
  the Node, browser, and desktop jobs begin.
- `pnpm install --frozen-lockfile`, generated-code drift checks, repository
  policy, Go vet, GolangCI, architecture checks, and Vue lint/typechecking pass
  from the release checkout.

## 2026-10-05 security CI repair evidence

This is a focused repair of Phase 1's requirement that quality commands run
locally and in CI. The existing application rebrand and feature changes are
outside this repair commit. ADR-0001, ADR-0002, and ADR-0017 still apply;
no architecture decision changed.

### Implemented

- Updated the pinned Astro, Vue, and sharp versions and affected transitive
  dependencies to remove the dependency audit's high and critical findings.
- Updated the desktop TLS dependency `rustls` from 0.23.43 to 0.23.45
  for RUSTSEC-2026-0285 using Cargo lockfile generation. No advisory exception
  or TLS behavior override was added.
- Kept undici overrides within their existing major versions. Added exact
  release-age exceptions only for the published cache-semantics and source-map
  security fixes; all other packages retain the ten-day release-age policy.
- Added a single historical Gitleaks fingerprint exception for a synthetic
  redaction fixture and annotated the fixture's current source line. Secret
  scanning, full-history scanning, and the audit threshold remain enabled.

### Verification

- Frozen pnpm installation and the CI audit threshold pass: zero high or
  critical findings, with one moderate advisory described below.
- Full-history Gitleaks scanning and the current redaction-source scan pass.
  Removing the fixture annotation causes the scanner to detect it again,
  verifying that the detection rule remains active.
- Go formatting, vet, lint (zero issues), architecture/repository checks,
  unit tests, race tests, empty-database migration, and vulnerability checks
  pass in the working checkout. Linux and Windows adapter compilation passes.
- Frontend lint, typechecking, and unit tests pass in the working checkout.
- The first hosted repair run passes Go quality/lint, all platform adapters
  and sidecars, frontend quality, application browser/visual checks, public
  site CI, and production site deployment. The Rust audit revealed the TLS
  advisory above; the patched lockfile has zero vulnerability findings.
- The CI-only master snapshot passes the site's typechecking, lint, 19 unit
  tests, production build, and content/output validation.
- API, manifest, SQL, and public-site generation reproduces the existing
  generated artifacts before this progress entry is added.

### Known limitations and follow-up

- The remaining moderate advisory, GHSA-rj75-hqrm-r3gf, affects
  `postcss-selector-parser` 6 through the site's code-highlighting CSS tooling.
  Its published fix is in major version 7; this repair does not force that
  major change underneath a dependency that still requires version 6.
- RustSec still reports six upstream unmaintained-package warnings and one
  upstream GLib unsoundness warning; these are informational in the existing
  audit policy and were not suppressed by this repair.
- Local browser checks initially encountered a cold daemon-build timeout and
  Astro's automatic agent background-preview mode. Browser verification uses
  a warm build cache and a foreground preview server; hosted CI remains the
  authority for the complete platform, browser, and desktop matrix.

### Manual verification

Run the CI and Public site CI workflows on the repair commit, then verify
that Security's JavaScript audit and full-history Secret scan jobs pass.
