---
title: "Phase 6: Native process runtime"
description: Implementation evidence for Switchyard product phase 6.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-17
---

## Implemented

- Added a typed process manifest with shell-free argv, contained working
  directories, project/process environment overlays, keychain references,
  stop timeouts, and bounded opt-in restart policy.
- Added deterministic adoption of reviewed `.switchyard/project.yml` files so
  explicit uv/npm process projects can complete the normal add/trust flow.
- Added dependency validation and deterministic topological start/reverse-stop
  ordering for multi-process projects.
- Added macOS/Linux process groups, separate inherited stdout/stderr pipes,
  graceful tree termination, forced escalation, start cancellation rollback,
  and parent-orphan reconciliation.
- Added durable `runs` and `run_processes` records with PID, process-group ID,
  executable, OS start time, cwd, run ID, restart count, outcome, and SHA-256
  identity fingerprint.
- Added exact fingerprint reconciliation across daemon restarts and a bounded
  npm-style launcher handoff window that never treats PID alone as ownership.
- Added non-zero exit reporting and explicit `on-failure` retry supervision.
- Added live process CPU/RSS metrics, bounded stdout/stderr logs, change-driven
  runtime events, and external TCP-listener classification using process and
  ancestor command metadata.
- Extended the generated manifest schema, OpenAPI Go/TypeScript clients,
  runtime HTTP contract, and CLI rendering for process observations.

## Files and modules added

- `internal/runtime/process`
- `internal/platform/sqlite/runs.go`
- `migrations/00004_native_process_runs.sql`
- `test/fixtures/uv-single-process`
- `test/fixtures/node-single-process`
- `test/integration/native_process_runtime_test.go`
- `test/integration/native_external_unix_test.go`
- `docs/architecture/native-process-runtime.md`

## Architecture decisions

No new ADR was required. The implementation realizes accepted ADR-0007. Run
intent and identity evidence are durable; current process state remains a live
derived observation. The driver owns platform APIs and credential lookup while
runtime domain/application packages remain OS- and persistence-neutral.

The current supported process-inspection dependency is
`github.com/shirou/gopsutil/v4` v4.26.5. Process groups are the Phase 6
macOS/Linux authority. Windows Job Objects remain Phase 18 work, consistent
with ADR-0015.

## Tests added

- Process manifest shell, secret-reference, binding, and dependency-cycle
  validation.
- Explicit portable-manifest discovery and accepted-proposal selection.
- Deterministic dependency plans and environment/keychain overlay precedence.
- SQLite run, member-fingerprint, restart-count, and terminal-outcome roundtrip.
- PID reuse rejection and exact external-listener classification.
- Real process-group tree stop, stdout/stderr capture, metrics, parent orphan,
  crash/exit code, opt-in restart, SIGTERM refusal/escalation, and cancellation
  rollback tests.
- Real uv and npm fixture onboarding/lifecycle tests with post-stop PID checks.
- Real externally launched uv listener recognition without ownership.

## Commands run and results

```text
go test ./internal/runtime/process -count=1
PASS: tree, orphan, crash, restart, escalation, cancellation, PID reuse

go test -tags=integration ./test/integration \
  -run 'TestUVAndNPM|TestNativeRuntimeRecognizesExternal' -count=1 -v
PASS: uv/npm lifecycle and external uv recognition in 4.924 seconds

SWITCHYARD_DOCKER_INTEGRATION=1 go test -tags=integration ./test/integration \
  -run TestComposeRuntimeLifecycleObservationLogsMetricsAndExternalRecognition \
  -count=1 -v
PASS: Compose regression lifecycle and cleanup in 22.39 seconds

make quality
PASS: generated drift, formatting, vet, golangci-lint, architecture checks,
frontend lint/typecheck/tests, all Go tests and race tests, migration smoke,
govulncheck, Playwright E2E/visual tests, production web build, packaged binary

GOOS=linux GOARCH=amd64 go build -trimpath -o /tmp/switchyard-phase6-linux-amd64 ./cmd/switchyard
GOOS=windows GOARCH=amd64 go build -trimpath -o /tmp/switchyard-phase6-windows-amd64.exe ./cmd/switchyard
PASS: Linux and Windows amd64 cross-builds
```

Packaged verification used a fresh SQLite database, private Unix socket, and
loopback address. The built binary completed add, trust, plan, durable start,
running status, stdout/stderr logs, metrics, daemon shutdown/restart, recovered
running status/metrics, durable stop, and final resource checks for the npm
fixture. A first run exposed an event/launcher handoff race; after the bounded
fingerprint-only reconciliation fix, the full flow succeeded. The final ledger
had zero active runs, one `stopped` outcome, and no TCP listener.

## Acceptance criteria status

- [x] The uv and npm fixtures start, report status, stream bounded logs, expose
  metrics, and stop without orphaned child PIDs.
- [x] A failed process reports its exact exit code and `failed` project state.
- [x] A stale/reused PID fails fingerprint verification, becomes
  `identity_lost`, and is never signalled as the original run.
- [x] Known shell executables and shell syntax in the executable field are
  rejected unless `shell: true` is explicit and previewed.
- [x] A matching port/process launched outside Switchyard is reported as
  `running_external` with no run ID or ownership claim.

## Known limitations

- Captured log buffers are current-daemon memory only. Persistent rotating,
  redacted log segments and post-restart history are Phase 7.
- Linux keychain lookup requires `secret-tool` and a working Secret Service.
  Windows credential resolution is not yet available.
- External recognition requires a declared TCP port plus matching executable
  or bounded ancestor command evidence. Services without that evidence remain
  stopped/unknown rather than being guessed.
- Windows currently has a compile-safe single-process termination fallback;
  Job Object tree ownership is Phase 18.
- Per-process network byte attribution is unavailable and is reported as zero.

## Deferred work

- Health scheduling, persistent/redacted logs, retention, and log follow over
  WebSocket are Phase 7.
- Port reservation/conflict intelligence and process-listener registry are
  Phase 8.
- Interactive PTY terminals are Phase 14.

## Manual verification

```bash
make build
switchyard add test/fixtures/uv-single-process
switchyard project trust uv-single-process --yes
switchyard plan start uv-single-process
switchyard start uv-single-process
switchyard status uv-single-process
switchyard logs uv-single-process --service web --tail 50
switchyard metrics uv-single-process --service web
switchyard stop uv-single-process
```
