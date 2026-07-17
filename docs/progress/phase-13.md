---
title: "Phase 13: Workspaces, dependencies, worktrees, and local routing"
description: Implementation evidence for Switchyard product phase 13.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-17
---

## Implemented

- Added a workspace domain with validated DAGs, member roles, health gates,
  rollback/continue policies, profiles, recipes, immutable execution snapshots,
  and explicit per-member progress.
- Added dependency-ordered start and reverse-order stop. Ready independent
  branches run concurrently within the selected profile ceiling; low-memory
  profiles provide dependency-closed subsets and bounded parallelism.
- Routed every child start/stop through the existing durable operation
  coordinator, retaining per-project serialization, cancellation, audit facts,
  idempotency, and workspace attribution.
- Added SQLite migrations 9 and 10 for workspace graphs/runs and registered
  project environments. Interrupted workspace runs reconcile before generic
  operation recovery without hiding already-running runtimes.
- Added explicit trusted Git worktree registration with stable environment IDs,
  unique Compose names, port-lease namespaces, exact leases, logical offsets,
  and friendly hostnames. Nested monorepo project paths are projected into each
  checkout rather than incorrectly using the Git root.
- Added private mode-`0600` Compose override generation for exact loopback port
  bindings, including `docker compose config` validation before lifecycle use.
- Added optional loopback-only HTTP `.localhost` routing with normalized host
  names, loopback target validation, forwarding-header removal, no-store
  responses, and explicit active/unavailable/conflict/disabled states.
- Added reviewed post-start recipes for URLs, terminals, editors, Codex, and
  Claude Code. Recipe failure is reported independently and never fabricated as
  lifecycle success.
- Added generated REST, Go, and TypeScript contracts plus CLI, MCP, and browser
  adapters. MCP member scope resolves environment IDs to their trusted base
  project before authorization, and data removal remains admin-confirmed.
- Added a responsive workspace builder, dependency graph, profile controls,
  execution progress, worktree member selection, and project Git-tab environment
  registration. TanStack Query owns server state and visual regression covers
  the complete graph/progress state.
- Split Phase 13 HTTP client methods into a cohesive transport file after the
  original client crossed the repository's 600-line review threshold.

## Architecture decisions

No new ADR was required. The implementation follows ADR-0001's modular-monolith
dependency direction, ADR-0004's shared generated transport contract,
ADR-0005's domain-owned SQLite boundaries, ADR-0006's Compose isolation,
ADR-0010's thin MCP adapter, ADR-0013's explicit permissions, and ADR-0014's
bounded/redacted responses.

Workspace coordination does not bypass runtime ownership. The workspace domain
plans and records graph execution; operations serialize mutations; runtime
drivers own lifecycle; environments own worktree identities; ports own exact
lease evidence; and routing owns only safe local HTTP forwarding.

## Safety properties

- Worktree registration is explicit and post-trust; deterministic discovery
  still executes no repository command.
- Every runtime worktree uses a distinct Compose and port namespace. Exact host
  ports are conflict-checked and observed before a route becomes active.
- Routing is disabled by default, binds only loopback, accepts only loopback
  HTTP targets, and returns `503` for unknown or unavailable hosts.
- Bulk stop preserves runtime data unless removal and confirmation are both
  present. MCP additionally requires destructive capability.
- Launch recipes are typed and reviewed; no generic shell recipe or MCP shell
  tool exists.

## Tests added

- DAG cycles, start/stop ordering, parallel branches, health gates and timeouts,
  rollback, continue/partial outcomes, cancellation, profiles, recipe timing,
  and data-preserving/destructive stop behavior.
- Workspace SQLite round trips, optimistic revisions, execution progress,
  restart recovery, and environment-member persistence.
- Worktree identity/allocation collisions, registration replacement, bare and
  removed checkouts, cancellation, exact leases, runtime route state, and
  nested monorepo subdirectory projection.
- Compose exact-port override permissions, validation, argument construction,
  and lifecycle binding evidence.
- Hostname/target validation, route conflicts, unavailable states, proxy header
  stripping, unknown-host rejection, and disabled routing.
- Workspace/environment HTTP handlers, CLI contracts, MCP discovery/scope/
  permission behavior, and browser workspace visual regression.

## Acceptance criteria status

- [x] A workspace starts dependencies in order and independent branches in
  parallel, bounded by its selected profile.
- [x] Rollback and continue behavior produce durable, visible per-project
  outcomes, including partial success and cancellation.
- [x] Multiple worktrees receive collision-resistant Compose names and distinct
  exact port leases; tests cover simultaneous allocations and lifecycle facts.
- [x] Friendly `.localhost` URLs resolve only to the matching active environment
  and safely reject conflicts, inactive targets, and unknown hosts.
- [x] Bulk stop preserves data by default and requires an explicit destructive
  request plus confirmation before removal.

## Verification evidence

`make quality` passed on 2026-07-16 after the final monorepo regression fix. It
covered generated-artifact drift, Go/Vue lint, architecture constraints,
TypeScript checks, every Go and web unit test, `go test -race ./...`, migration
from an empty database through schema version 10, `govulncheck` with no known
vulnerabilities, four real-runtime browser E2E tests, eight visual-regression
tests, and production Go/web builds.

The release binary then ran against an isolated `/tmp` data directory with API
and routing listeners on separate loopback ports. The real CLI deterministically
scanned and explicitly trusted the Compose fixture, registered its Git worktree,
created an environment-backed low-memory workspace, and read both records back
after a full daemon restart. The route listener returned `503` for an unknown
`.localhost` host. This live run exposed and verified the fix for nested
monorepo projects: the environment reconciled from the Git root to
`test/fixtures/compose-runtime`, observer errors stopped, and the corrected path
survived restart.

## Known limitations and deferred work

- Local TLS and certificate management remain separate from the opt-in HTTP
  router, as required by the phase scope guard.
- Routing selects a validated HTTP target from exact active environment facts;
  non-HTTP bound services remain visible but are not proxyable.
- Worktrees are observed and registered but never created, removed, or checked
  out by Switchyard. Repository mutation remains outside this phase.
