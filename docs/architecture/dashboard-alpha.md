# Dashboard alpha

Phase 9 turns the browser client into the primary visual adapter over the
Switchyard control plane. The dashboard does not own runtime policy: it reads
generated OpenAPI contracts, submits typed operations, and follows the same
durable event and log streams used by other clients.

## Route map

| Route | Responsibility |
|---|---|
| `/` and `/projects` | Project overview, filtering, sorting, tags, recent access, and aggregate status |
| `/projects/:id` | Runtime, health, logs, resources, Git, ports, trusted actions, and manifest provenance |
| `/ports` | Declared, reserved, and bound port evidence and conflicts |
| `/resources` | Current cross-project CPU and memory samples with attribution warnings |
| `/logs` | Bounded cross-project log search and filtering |
| `/discovery` | Deterministic scan, evidence review, validation, and trust approval |
| `/settings` | Daemon identity, host capabilities, and browser-local display preferences |

Workspace and agent routes deliberately remain honest feature shells until
their roadmap phases. The browser UI is stable before the thin Tauri shell is
introduced.

## State ownership

TanStack Query owns all daemon-derived server state. Query keys are scoped by
resource and project, and the event stream invalidates operation state plus
project runtime and health observations at a bounded frequency. Ordinary
polling remains as a fallback when WebSocket events are delayed or unavailable.
The browser bounds dashboard fan-out, log history, and concurrent project
requests to avoid unbounded work in large catalogs.

Browser-only preferences and recent project access use guarded local storage.
They degrade to in-memory defaults when storage is denied. No browser state is
treated as daemon authority.

## Command and operation boundary

Views may select only generated action enums or trusted action identifiers.
They never assemble an executable, argument list, Compose command, or shell
string. Mutations return durable operation records which appear in the global
operation toast and drawer. The drawer reports queued, running, terminal, and
cancellation-requested states and delegates cancellation to the operation API.

This preserves the architecture boundary:

```text
Vue intent -> generated HTTP client -> application use case
                                      -> durable operation
                                      -> trusted runtime/action adapter
```

## Degraded behavior

Each route distinguishes loading, empty, partial, stale, disconnected,
degraded, and failed observations. Docker failure is a capability failure, not
a daemon failure: process projects, discovery, Git, settings, logs already in
the archive, and other local workflows remain available. Container storage is
explicitly labeled shared when Docker cannot provide reliable per-project
attribution.

## Accessibility and visual evidence

The application provides a skip link, visible focus treatment, semantic
landmarks, named dialogs, keyboard-operable tabs and command palette, focus
restoration, reduced-motion support, and responsive layouts. Playwright runs an
axe WCAG 2/2.1 A/AA serious-and-critical gate on the live dashboard. Approved
visual baselines cover populated, empty, degraded, conflict, and narrow states.

The visual fixtures intercept API traffic only in visual tests. End-to-end
tests use the packaged daemon and exercise real deterministic onboarding,
native process lifecycle, and Docker Compose lifecycle behavior.
