# Docker Compose runtime

The Compose runtime implements [ADR-0006](../adr/0006-docker-compose-runtime.md):
the installed `docker compose` plugin owns lifecycle semantics, while the
official Moby Engine client owns observation and streams. Both receive the same
resolved Docker connection and normalized Compose project identity.

## Responsibility boundaries

| Component | Responsibility |
|---|---|
| Runtime application service | Resolve trusted projects, select a driver, expose plans/observations/streams |
| Catalog source | Convert only accepted effective manifests into driver-neutral runtime input |
| Context resolver | Honor an explicit manifest context, `DOCKER_HOST`, or the active Docker context |
| Config reader | Run `docker compose config --format json` and retain only project/service identity |
| Command builder | Produce shell-free, reviewable lifecycle plans |
| Executor | Run one validated plan with bounded output and cancellation |
| Engine observer | Inspect containers selected by canonical Compose labels |
| Log/metric sources | Attach project, service, container-run, and stream identity |
| Event watcher | Filter Engine events by project label and trigger targeted live reconciliation |

Transport handlers and Cobra commands call runtime application use cases. They
do not import the Compose adapter or Moby SDK. Docker disconnection becomes an
`unknown` observation or a typed unavailable error and never terminates the
daemon.

## Identity and ownership

Membership requires both `com.docker.compose.project` and
`com.docker.compose.service`. One-off containers are excluded. Names are used
only for display and never to infer project membership.

The daemon marks containers as Switchyard-originated only after a lifecycle
command succeeds and a subsequent observation reconciles their Engine IDs.
After daemon restart, existing containers are honestly reported as external
until a new managed lifecycle action establishes current-session ownership.

## Lifecycle plans

| Action | Compose command suffix | Risk | Volume behavior |
|---|---|---|---|
| start | `up --detach` | safe | preserve |
| stop | `stop` | caution | preserve containers and volumes |
| restart | `restart` | caution | preserve |
| pause | `pause` | caution | preserve |
| unpause | `unpause` | safe | preserve |
| rebuild | `up --detach --build --force-recreate` | caution | preserve named volumes |
| teardown | `down [--volumes]` | destructive | exactly matches the previewed flag |

Every command uses an argument array, a trusted working directory, canonical
Compose file paths contained by the project root, and a two-second `os/exec`
wait bound after cancellation. Teardown requires an explicit CLI confirmation;
its plan endpoint is side-effect-free and therefore does not require mutation
credentials.

## Observation and streams

Project state is derived on demand from declared topology, container state, and
Docker health. Service observations include health, exit code for stopped
containers, restart count, timestamps, image, container ID, and published
ports. Engine API negotiation is enabled and server/API versions are reported.

Docker logs preserve stdout/stderr, the original line, timestamp, project ID,
service ID, and a container-derived run identity. Current metrics expose CPU,
memory, and aggregate network counters. Persistence, redaction, retention, and
WebSocket follow behavior belong to Phase 7; Phase 5 intentionally supplies the
bounded live driver streams they will consume.

The integration fixture builds a small static Go server into a `FROM scratch`
image. It does not require registry credentials or network access and always
cleans its namespaced containers, image, network, volume, and temporary binary.
