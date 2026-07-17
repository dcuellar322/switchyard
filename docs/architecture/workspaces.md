---
title: Workspace and worktree orchestration
description: Dependency graphs, coordinated lifecycle, port leases, environments, and local routing.
category: concept
audience: [user, contributor, integrator]
since: 1.0.0
lastVerified: 2026-07-17
---

Phase 13 adds coordination without turning the workspace domain into a second
runtime engine. The workspace application service validates and schedules a
graph; each member lifecycle still enters the durable operation coordinator and
the existing runtime application boundary.

## Domain boundaries

- `workspace` owns graph definitions, roles, dependencies, profiles, launch
  recipes, execution progress, failure policy, and coordinated ordering.
- `environments` owns durable registration and isolation identity for trusted
  Git worktrees.
- `routing` owns the optional hostname registry and loopback HTTP proxy.
- `sourcecontrol` supplies read-only Git worktree observations.
- `runtime`, `operations`, and `ports` remain authoritative for lifecycle,
  serialization, and exact binding evidence.

These contexts communicate through consumer-owned application interfaces.
None reads another context's SQLite tables.

## Graph execution

A workspace is a validated directed acyclic graph. An edge from application to
database means the database must start first and stop last. Independent ready
branches run in parallel up to the selected profile's `maxParallel` limit.
Profiles may select a subset only when its dependency closure is complete.

Health-gated members do not release their dependants until readiness succeeds
or the configured timeout expires. Cancellation stops scheduling new work and
records honest per-member cancellation or completed outcomes.

Two failure policies are explicit:

- `rollback` stops members started by the failed run in reverse dependency
  order;
- `continue` runs every independent branch it still can and finishes as
  partial success when at least one branch fails.

Every outer workspace run and child project operation is durable and carries
workspace attribution. Child lifecycle work uses the same per-project lock as
direct CLI, browser, and MCP operations.

## Worktree isolation

Worktrees are never registered during deterministic discovery. After project
trust, an explicit registration reads `git worktree list --porcelain` and
persists one stable environment per checkout. For a project in a monorepo,
Switchyard computes its path relative to the primary Git worktree and projects
that subdirectory into every alternate checkout.

Each environment receives:

- a path-sensitive opaque ID;
- a collision-resistant Compose project name;
- a distinct port-lease namespace and stable logical offset;
- exact per-declared-port leases selected through the port registry;
- a stable `.localhost` hostname.

Compose starts use a private mode-`0600` override with exact loopback host-port
bindings. The override is validated with `docker compose config` before use.
Logical offsets are never treated as proof that a concrete port is free.

An environment is marked active only after its expected exact TCP binding is
observed. Stop clears its route target but preserves registration and leases.
Removed or bare worktrees remain explicit unavailable facts rather than fake
runnable states.

## Local routing

Routing is off by default. `--routing-address` must name a loopback listener.
The proxy accepts normalized `.localhost` hostnames and loopback `http://`
targets only, strips incoming forwarding headers, disables response caching,
and never exposes a general forward proxy.

The route registry distinguishes active, unavailable, conflicting, and
disabled states. Unknown or inactive hostnames return `503`. HTTPS and local
certificate management are outside this phase.

## Recipes and destructive actions

Recipes are reviewed, typed post-start actions for opening an HTTP URL,
terminal, editor, or supported coding agent at a trusted member location. They
run only after a successful workspace start and report their own failures.

Bulk stop preserves containers, volumes, and other runtime data by default.
Data removal must be present in the reviewed request and separately confirmed;
CLI and MCP enforce the corresponding destructive capability.

## Recovery

Workspace run recovery occurs before generic operation recovery. An interrupted
run is closed as failed, partial, or cancelled from its persisted member facts;
already-running runtimes remain visible and are not silently stopped. Registered
environments, allocations, routes, and the last workspace execution are rebuilt
from durable state on daemon restart.
