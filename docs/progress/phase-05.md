# Phase 5: Docker Compose runtime

## Implemented

- Added driver-neutral runtime plans, observations, logs, metrics, and event
  contracts under `internal/runtime/domain` and application orchestration under
  `internal/runtime/application`.
- Added trusted catalog-to-runtime resolution; pending projects cannot execute
  repository-derived runtime configuration.
- Added Compose config normalization, root-contained file construction, Docker
  context resolution, API negotiation, and clear disconnected-engine behavior.
- Added shell-free start, stop, restart, pause, unpause, rebuild, and teardown
  plans using the installed Compose CLI.
- Added trusted optional-profile selection for start/rebuild. Project-wide stop
  and teardown use Compose's all-profile selector so optional services cannot
  be orphaned.
- Added Engine SDK observation keyed exclusively by canonical Compose labels,
  including external-origin recognition, health, exit state, restart count,
  container metadata, and published ports.
- Compose observation, log, and metric membership is intersected with the
  normalized default-profile service set, so stale containers from inactive
  profiles cannot degrade or pollute the active project runtime. Active
  optional-profile containers remain visible.
- A successful stop is treated as deliberate lifecycle intent, preventing a
  forced container exit code from turning a stopped project into `failed`.
- Added bounded Docker stdout/stderr streaming with service/run identity and
  current CPU, memory, and network sampling.
- Added dynamically maintained Engine event subscriptions; a project-labelled
  event triggers targeted live observation and a durable `runtime.observed`
  event.
- Added REST/OpenAPI and CLI status, plan, operation, log, and metric contracts.
  Runtime mutations use the durable per-project operation coordinator.
- Added a deterministic offline Docker fixture that cross-compiles a static
  health server and builds a scratch image.

## Files and modules added

- `internal/runtime/domain`
- `internal/runtime/application`
- `internal/runtime/compose`
- `internal/bootstrap/runtime_events.go`
- `internal/transport/httpapi/runtime_handler.go`
- `internal/transport/cli/runtime_commands.go`
- `test/fixtures/compose-runtime`
- `test/integration/compose_runtime_test.go`
- `docs/architecture/docker-compose-runtime.md`

## Architecture decisions

No new ADR was required. The implementation realizes accepted ADR-0006. The
supported Docker modules are `github.com/moby/moby/client` and
`github.com/moby/moby/api`; the deprecated pre-Docker-29 monolithic SDK path is
not used.

The Compose CLI remains the lifecycle authority. The Engine SDK remains the
observation/stream authority. Runtime state is derived rather than persisted as
truth, and current-session ownership is conservative.

## Tests added

- Runtime action validation and application driver routing.
- Dynamic watcher identity attachment.
- Compose action/risk/effect command construction and root-containment checks.
- Trusted profile allowlisting, all-profile stop/teardown commands, and
  deliberate-stop state derivation.
- Context/config normalization and deterministic service ordering.
- Label-only membership, one-off exclusion, external recognition, disconnected
  observations, inactive-profile exclusion, state derivation, and
  published-port mapping.
- Docker log demultiplexing, timestamp/level handling, identity preservation,
  and CPU/network calculations.
- Event label filtering and durable operation progress-state compatibility.
- HTTP durable-operation and side-effect-free plan authorization behavior.
- Full opt-in Docker integration lifecycle with unconditional cleanup.

## Commands run and results

```text
make quality
PASS: generated drift, formatting, vet, golangci-lint, architecture checks,
frontend lint/typecheck/tests, Go tests and race tests, migration smoke,
govulncheck, Playwright E2E/visual tests, production web build, packaged binary

go test ./internal/runtime/... ./internal/bootstrap ./internal/transport/httpapi ./internal/transport/cli
PASS

SWITCHYARD_DOCKER_INTEGRATION=1 go test -tags=integration ./test/integration \
  -run TestComposeRuntimeLifecycleObservationLogsMetricsAndExternalRecognition \
  -count=1 -v
PASS: full lifecycle and cleanup in 22.03 seconds

GOOS=linux GOARCH=amd64 go build -trimpath -o /tmp/switchyard-linux-amd64 ./cmd/switchyard
GOOS=windows GOARCH=amd64 go build -trimpath -o /tmp/switchyard-windows-amd64.exe ./cmd/switchyard
PASS: Linux and Windows amd64 cross-builds
```

Packaged CLI/daemon verification used a fresh SQLite data directory and private
Unix socket. It completed add, trust, plan, durable start, healthy status,
JSONL logs, metrics, pause, unpause, restart, rebuild, stop, restart-from-stop,
and teardown-with-volumes. An initial start exposed an invalid progress-state
word, which was corrected and locked down by a regression test; every rerun
operation succeeded. The final fixture resource query returned no containers,
networks, or volumes.

## Acceptance criteria status

- [x] A Compose fixture starts, becomes healthy, stops, restarts, rebuilds, and
  tears down through both the driver integration and packaged durable path.
- [x] Stop preserves containers/volumes; teardown removes exactly the resources
  shown in its plan, including opt-in volumes.
- [x] A Compose project started outside Switchyard is reported as
  `running_external` after health convergence.
- [x] Membership uses canonical Compose project/service labels and excludes
  one-off containers; no container-name parsing determines membership.
- [x] Docker disconnection produces a bounded unknown observation or typed
  unavailable result and cannot crash the daemon or block other drivers.

## Known limitations

- SSH Docker contexts are rejected with an actionable error because the Engine
  SDK does not natively load Docker CLI SSH connection helpers.
- Contexts that explicitly disable TLS verification are refused. Local Unix and
  verified TCP/TLS contexts are supported.
- Runtime ownership is intentionally session-local until durable run identity is
  added with native-process run records.

## Deferred work

- Persistent rotating log segments, redaction, retention, WebSocket log follow,
  historical metrics, and health scheduling are Phase 7.
- Native process runtimes and persistent run fingerprints are Phase 6.
- Rich runtime UI states are Phase 9.

## Manual verification

```bash
make build
switchyard add test/fixtures/compose-runtime
switchyard project trust compose-runtime --yes
switchyard plan teardown compose-runtime --volumes
switchyard start compose-runtime
switchyard status compose-runtime --json
switchyard logs compose-runtime --service web --jsonl
switchyard metrics compose-runtime --service web --json
switchyard stop compose-runtime
switchyard teardown compose-runtime --volumes --yes
```
