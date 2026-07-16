# Phase 7: Health, logs, and event experience

## Implemented

- Added manifest declarations and generated JSON Schema support for HTTP status
  and JSON assertions, TCP, process, Docker, shell-free command, and composite
  `all`/`any` health checks.
- Added initial delay, interval, stable jitter, timeout, retry, severity,
  required-readiness, cancellation, loopback enforcement, and composite-cycle
  validation.
- Added a cancellable health scheduler with bounded project concurrency, latest
  sample persistence, 24-hour sample pruning, stale/disconnected states, and
  manifest-change rescheduling.
- Added readiness gating after successful start/restart/rebuild execution.
  Required-health failure fails the operation without stopping or claiming the
  runtime stopped.
- Added runtime-state overlay for `starting` while health is unknown and
  `degraded` when required or non-informational health checks fail.
- Added one canonical redact-before-sink log pipeline for native processes and
  Docker Compose, including built-in credential patterns, repeated user regex
  flags, and in-memory registration of resolved keychain values.
- Added bounded per-service rings, rotating private NDJSON files, SQLite segment
  and line metadata, monotonic sequence cursors, digest deduplication, checksums,
  staged-deletion recovery, age retention, and disk-cap retention.
- Added persisted log queries by service, time, run, and operation plus plain
  text and NDJSON exports through generated HTTP clients and the CLI.
- Added `/ws/v1/logs` with subscribe-before-replay ordering, cursor resume,
  replay/live deduplication, overflow recovery, browser authentication, and
  exact same-origin enforcement.
- Added durable native-run operation IDs and Compose operation correlation so
  persisted output can be traced to the lifecycle action that initiated it.
- Added a responsive project diagnostics UI with runtime services, health
  severity/readiness, stale/disconnected banners, redaction labels, persisted
  log preview, and cursor-resumable live updates.

## Files and modules added

- `internal/observability/domain`
- `internal/observability/application`
- `internal/observability/adapters`
- `internal/platform/sqlite/health.go`
- `internal/platform/sqlite/log_store.go`
- `internal/platform/sqlite/log_query.go`
- `internal/platform/sqlite/log_retention.go`
- `internal/transport/websocket/logs.go`
- `migrations/00005_observability.sql`
- `web/src/domains/projects/components/ProjectDiagnostics.vue`
- `web/src/domains/projects/composables/useProjectLogStream.ts`
- `docs/architecture/observability.md`

## Architecture decisions

No new ADR was required. The implementation realizes accepted ADR-0014:
runtime drivers remain raw observation adapters, while the observability
application boundary owns redaction, persistence, retention, health policy,
and client replay. SQLite stores indexes and segment metadata rather than log
payloads; private NDJSON files remain the content authority.

The operation coordinator still owns lifecycle completion. Its runtime executor
now runs required readiness after driver execution, preserving durable failure
and cancellation semantics without adding health policy to a runtime driver.

## Tests added

- Every health-check type, JSON assertion, port substitution, loopback defense,
  duplicate IDs, composite membership/cycles, composite modes, retry
  re-observation, required failure, and operation readiness gating.
- Redaction of bearer tokens, secret assignments, credential URLs, common access
  keys, resolved keychain values, user patterns, and log attributes.
- One secret canary across live subscribers, persisted files, query/diagnostic
  reads, and NDJSON export.
- Segment rotation, checksums, age/disk retention, and replay-tail
  deduplication.
- Native and Compose operation-to-log correlation.
- WebSocket reconnect replay with deliberate live overlap.
- HTTP degraded-state derivation and browser same-origin denial for both event
  and log sockets.
- Vue degraded/disconnected/empty/redacted states and browser cursor resume.
- Playwright trust-to-diagnostics E2E coverage against the real daemon.

## Verification evidence

```text
go test ./...
PASS: all Go packages, including real process groups and WebSocket listeners

make lint
PASS: gofmt, go vet, golangci-lint, architecture check, frontend ESLint

pnpm --dir web typecheck
pnpm --dir web test
PASS: 4 unit suites, 5 tests

pnpm --dir web test:e2e
PASS: 2 browser flows, including project trust and diagnostics

pnpm --dir web test:visual
PASS: approved system visual baseline

make build
PASS: production Vue bundle and packaged Go binary

make quality
PASS: generated-code drift, lint, typechecking, unit and race tests, fresh
database migration, vulnerability scan, isolated browser E2E, visual baseline,
and production build
```

Packaged verification used a fresh temporary database and a real Python HTTP
process. The start operation took 1.74 seconds and reached `succeeded` only
after its required HTTP 200 plus `$.ready == true` assertion passed. Runtime
status remained `running`; stop completed with no remaining listener. The
process emitted `token=phase7-secret-canary`. CLI query, operation-filtered
NDJSON export, and the mode-0600 segment all contained
`token=[REDACTED]`; the canary appeared in none of them. The run and log entry
both retained the initiating operation ID. The same database reported schema
version 5.

## Acceptance criteria status

- [x] Start/restart/rebuild complete only after every required check passes.
- [x] Health failure derives `degraded` while retaining the running runtime
  observation and ownership evidence.
- [x] Rings, segment rotation, retention age, and aggregate disk caps are
  bounded and configurable.
- [x] Secret fixtures are redacted before live delivery, persistence, query,
  export, or diagnostic consumption.
- [x] Browser reconnect resumes from a monotonic sequence without duplicate or
  missing-state confusion.

## Known limitations and deferred work

- Closed-segment compression, full-text indexing, cross-project search, and
  saved log filters remain intentionally deferred.
- Historical metrics/downsampling and storage-attribution estimates remain a
  later observability slice.
- Health checks target local resources only. Remote probes require a future
  explicit trust and network policy rather than bypassing loopback validation.
- Docker logs retain start/rebuild operation correlation in daemon memory;
  native process run correlation survives daemon restart through the run
  ledger.
