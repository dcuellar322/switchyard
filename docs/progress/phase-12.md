# Phase 12: Resource and storage intelligence

## Implemented

- Added one process-wide, cancellable resource sampler with a four-project
  concurrency ceiling, per-project deadlines, active-runtime filtering, and
  idle maintenance throttling. Stopped projects are inspected for state but do
  not invoke driver statistics, health reads, or history writes.
- Added project/service aggregation across every Compose replica and every
  verified managed process-tree member. Samples carry explicit CPU, memory,
  network, disk, health, process-count, restart, partial, and availability
  evidence so a failed read is never rendered as a believable zero.
- Added SQLite migration 8 with atomic/idempotent raw samples, availability-
  aware one-minute and fifteen-minute rollups, aligned tier pruning, bounded
  history queries, and database/WAL/SHM/log/history footprint reporting.
- Added manifest-backed CPU, memory, and storage budgets with warnings only
  after three consecutive fresh samples exceed the configured threshold.
- Added a read-only Docker SDK storage inspector for containers, images,
  volumes, and build cache. It uses canonical Compose project labels and live
  references, caches Engine disk usage for two minutes, and times observations
  out after five seconds.
- Added explicit `exclusive`, `shared`, `estimated`, and `unknown` attribution
  with a human-readable reason on every resource. Image layers and build cache
  are never presented as exact project-exclusive bytes.
- Added a non-executable cleanup preview containing stable exact Engine IDs,
  per-resource attribution, known reclaimable bytes, and unknown-size counts.
  No prune/delete application port, HTTP operation, CLI command, or MCP tool
  exists.
- Added generated aggregate resource, storage inventory, cleanup preview, and
  bounded history REST/TypeScript contracts.
- Replaced the dashboard fan-out with one resource overview query, separate
  cached storage query, retained time-series charts with visible gaps and an
  accessible table, project/service consumers, sustained warnings, retention
  facts, and Switchyard's own footprint.
- Replaced the project Storage placeholder with real project-scoped resource
  navigation. Project overview and service rows now distinguish unavailable
  CPU/memory and link into durable `?project=` history/storage context.
- Added daemon flags for sample interval, each metric retention tier, and the
  maximum history response size.

## Architecture decisions

No new ADR was required. This phase implements ADR-0005's SQLite ownership,
ADR-0006's Docker SDK observation and canonical-label attribution, ADR-0007's
PID-reuse-resistant process ownership, and ADR-0014's bounded retention model.

Resource intelligence remains in the observability bounded context. Runtime
drivers emit raw provider-neutral samples; the observability application layer
owns aggregation, budgets, retention policy, and read models. SQLite and Docker
remain consumer-owned adapters. The Docker storage interface intentionally has
no mutation method.

## Retention and overhead targets

Default metric policy:

| Tier | Interval | Retention | Maximum rows per project/service scope |
|---|---:|---:|---:|
| Exact | 10 seconds | 1 hour | 360 |
| Minute | 1 minute | 24 hours | 1,440 |
| Quarter hour | 15 minutes | 30 days | 2,880 |

History responses contain at most 1,000 points. Log retention remains seven
days or 256 MiB by default. Tier maintenance upserts only the current/new
bucket for an established scope, completes rollups before pruning their source,
and runs at most every fifteen minutes while every project is idle.

The documented Phase 12 idle targets are:

- one sampler ticker for the entire daemon;
- at most four concurrent project observations;
- zero driver-statistics and metric-history writes for stopped projects;
- a 50-stopped-project in-memory collection below 1 ms/op and 64 KiB/op on the
  reference development host;
- an empty live daemon below 1% of one CPU core and 100 MiB resident memory
  after startup settles.

`BenchmarkResourceSamplerFiftyIdleProjects` measured `88,980 ns/op`, `38,644
B/op`, and `215 allocs/op` on the Intel i9-8950HK reference host. An isolated
release daemon with no registered projects settled at `0.0%` CPU and `22.8
MiB` RSS after 32 seconds on that host.

## Tests added

- Sampler aggregation, max-four concurrency, cancellation, project failure,
  stopped-project no-write behavior, maintenance throttling, sustained/fresh
  budgets, bounded history selection, cleanup-preview safety, and the 50-idle
  benchmark.
- Active/stopped runtime adaptation, health latency, per-field availability,
  partial observations, and every runtime state.
- Canonical Docker attribution, shared images/volumes, remote/unknown volumes,
  build cache, disconnected Engine behavior, cache expiry, SDK options, and
  client closure.
- SQLite atomic/idempotent writes, availability-aware rollups, repeated
  maintenance, retention boundaries, bounded ordering, and footprint facts.
- Verified native process-tree metrics, partial member failure, service
  filters, PID reuse, and ended-run exclusion.
- Compose replica samples, disk accounting, restart/process counts, partial
  replica failure, and availability evidence.
- Resource HTTP contract/query/error mapping and the absence of an executable
  cleanup result.
- Resource loading, error, empty, partial, Docker-disconnected, filtering,
  exact preview, history control, accessible table, URL selection, and project
  Storage-tab flows.

## Acceptance criteria status

- [x] Project and service rows identify current available CPU and memory across
  Compose replicas and managed process trees.
- [x] Every storage resource and summary discloses exclusive, shared,
  estimated, or unknown attribution.
- [x] Cleanup preview lists exact IDs and known/unknown reclaimable estimates
  and has no execution capability.
- [x] Raw and downsampled history is transactionally rolled up, pruned to
  configured aligned tiers, and bounded at the response boundary.
- [x] Automated idle sampling meets the documented latency, allocation,
  concurrency, no-statistics, and no-write targets.

## Verification evidence

`make quality` passed on 2026-07-16, covering generated-artifact drift, Go and
Vue lint, architecture constraints, TypeScript checks, all Go and web unit
tests, `go test -race ./...`, schema migration to version 8, `govulncheck`, four
real-runtime browser E2E tests, seven visual-regression tests, and production
Go/web builds.

For live verification, the release binary ran against an isolated data
directory and Unix socket for 32 seconds. It measured `0.0%` CPU and `22.8 MiB`
RSS, below both idle targets. A request to `/api/v1/resources` over that socket
returned the empty-project overview, actual Docker storage inventory,
Switchyard footprint, default retention policy, and explicit attribution
warning without starting or mutating any project.

## Known limitations and deferred work

- Cleanup execution and resource throttling are intentionally absent. A future
  mutation must add a new explicit authorization, preview, audit, and race-
  revalidation use case; it cannot extend this read-only inspector implicitly.
- Docker image and build-cache records do not expose enough layer ownership for
  exact project totals. Those records remain shared, estimated, or unknown.
- Network and disk values are cumulative provider counters. The retained chart
  exposes exact observed values and gaps; rate derivation with reset metadata
  can be added as a later coherent analytics slice.
