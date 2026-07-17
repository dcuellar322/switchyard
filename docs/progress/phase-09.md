# Phase 9: Dashboard alpha

## Implemented

- Replaced the walking-skeleton page with a responsive command-center shell,
  primary navigation, host status, command palette, and global operation UI.
- Added dashboard, project detail, resources, logs, onboarding, ports, and
  settings routes using Vue Router and TanStack Query server state.
- Added project search, runtime/status sorting, tag filtering, guarded recent
  access, bounded fan-out, and current aggregate status cards.
- Added project runtime, health, service, resource, log, Git, port, manifest,
  and trusted-action surfaces without constructing commands in the client.
- Added operation toasts, a progress drawer, cancellation, event-driven query
  invalidation, polling fallback, loading and empty states, partial-data
  warnings, and Docker-disconnected behavior.
- Added a host observation endpoint for CPU, memory, Docker storage, explicit
  attribution, timeout warnings, and short-lived request coalescing.
- Corrected Compose ownership timing and reconciliation under concurrent
  observations so browser-started projects are not mislabeled external.
- Added skip navigation, focus-visible styling, keyboard tabs and command
  palette, focus restoration, reduced motion, and a WCAG axe gate. The shared
  secondary-text token now meets AA contrast.
- Added deterministic populated, empty, narrow, degraded, and port-conflict
  visual baselines plus real daemon browser flows for Compose and native
  process projects.
- Replaced the settings placeholder with a revisioned durable control-plane
  editor for approved roots, ports, retention, tools, agent permissions, AI
  adapters, and appearance. The split panels expose loading, disconnected,
  validation, optimistic-conflict, saved, and restart-required states, while
  the generated API and CLI round-trip the same document.

## Files and modules added

- `internal/system/application/host.go`
- `internal/platform/host`
- `web/src/app/components`
- `web/src/domains/dashboard`
- `web/src/domains/logs`
- `web/src/domains/operations`
- `web/src/domains/resources`
- `web/src/domains/projects/views/ProjectDetailView.vue`
- `web/src/domains/system/views/SettingsView.vue`
- `web/src/router.ts`
- `web/src/queryClient.ts`
- `web/tests/helpers/alphaMocks.ts`
- `docs/architecture/dashboard-alpha.md`

## Architecture decisions

No new ADR was required. The implementation realizes ADR-0003's embedded web
application, ADR-0004's generated REST/WebSocket boundary, ADR-0006's honest
Compose ownership, ADR-0009's browser-first stability guard, ADR-0013's local
session security, and ADR-0014's bounded redacted logs.

Server state belongs to TanStack Query. The event stream is an invalidation
signal rather than a second client-side source of truth. Product settings are
revisioned daemon state; only ephemeral browser recency remains guarded local
state. Runtime and action execution continue
through typed server operations; Vue components do not know command syntax.

## Tests added

- Shell routing, command palette filtering and keyboard execution, focus and
  tab navigation, Docker-unavailable project controls, and typed lifecycle
  intent.
- A 1,000-entry log profile asserting a bounded 500-entry result within the
  rendering data budget.
- Host observation, Docker warning, cache coalescing, HTTP contract, and
  Compose concurrent-ownership regression tests.
- Live browser onboarding, settings, axe WCAG A/AA, native process start/log/
  stop, and real Compose start/attribution/stop flows.
- Settings domain, SQLite migration, full-document compare-and-swap, redacted
  audit, root-policy, CLI export/apply, restart-effect, and Vue interaction
  tests.
- Six deterministic visual scenarios at desktop and narrow viewports.

## Verification evidence

```text
go test ./internal/runtime/compose -count=1
PASS

SWITCHYARD_DOCKER_INTEGRATION=1 go test -tags=integration ./test/integration \
  -run TestComposeRuntimeLifecycleObservationLogsMetricsAndExternalRecognition
PASS: lifecycle, ownership, logs, metrics, pause, restart, rebuild, volumes,
and external recognition against Docker Engine

pnpm --dir web typecheck
pnpm --dir web lint
pnpm --dir web test
PASS: component, state, keyboard, and bounded-log tests

pnpm --dir web exec playwright test --project=visual
PASS: 6 deterministic visual baselines

pnpm --dir web exec playwright test --project=e2e
PASS: live daemon, onboarding, native process, and Compose workflows
```

`make quality`
PASS: generated-code drift, lint, unit, race, migration, vulnerability,
browser, visual, production web, and production binary build checks.

## Acceptance criteria status

- [x] Dashboard and project detail align with the approved visual reference at
  the target and narrow viewports.
- [x] Primary navigation, command palette, tabs, discovery, settings, and
  operations are keyboard accessible and pass the serious/critical axe gate.
- [x] UI components submit only generated runtime enums or trusted action IDs;
  no component builds runtime commands.
- [x] Docker-unavailable states retain process, discovery, Git, logs, settings,
  and daemon functionality with explicit capability warnings.
- [x] The alpha manages real Compose and process projects through the same
  browser operation model end to end.
- [x] Browser, CLI, MCP defaults, and daemon composition consume the same
  durable settings document without persisting provider secrets.

## Known limitations and deferred work

- Resource history and time-series charts belong to the observability
  expansion phase; this shell intentionally presents current bounded samples.
- The cross-project log explorer searches bounded recent project archives;
  full-text indexing remains deferred.
- Workspace and agent routes are explicit roadmap shells until their owning
  phases. Tauri packaging has not started.
- Host Docker storage is labeled shared because Engine accounting cannot be
  reliably assigned to one Compose project.
