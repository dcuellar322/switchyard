---
title: Engineering conventions
description: Naming, error, logging, testing, migration, API, and generation rules for contributors.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-17
slug: docs/conventions
---

`AGENTS.md` and the accepted ADRs are normative. This document makes the
cross-cutting implementation conventions explicit.

## Naming

- Use product glossary terms exactly; do not invent synonyms for core entities.
- Go packages use short domain nouns and avoid stutter. Exported names describe
  domain meaning, not implementation technology.
- IDs are stable opaque values. Display names and paths are not primary keys.
- APIs use lower-camel JSON fields, kebab-free path segments, and resource
  nouns. Database names use `snake_case`.
- Avoid root packages or modules named `utils`, `common`, `helpers`, `manager`,
  or `service` without a specific domain responsibility.

## Errors

- Domain/application errors have stable codes and safe user summaries.
- Wrap infrastructure errors with operation context while preserving causes.
- Transport adapters map errors to RFC 9457-style problem details; handlers do
  not manufacture business outcomes.
- Never expose secrets, environment values, raw SQL, or unsafe command details.
- Cancellation and deadline errors remain distinguishable from failures.

## Logging

- Use `log/slog` with UTC timestamps and structured attributes.
- Include component and error code; include operation/project/service IDs when
  available.
- Do not log secrets, authentication headers, raw provider prompts, environment
  values, or command arguments that may contain secrets.
- User-visible process logs pass through redaction before display, persistence,
  export, or diagnostics.

## Testing

- Domain tests cover invariants and state transitions.
- Application tests use focused fakes owned by the consuming package.
- Adapter tests exercise real SQLite, Git, process, Docker, or platform
  behavior where applicable.
- Cover success, validation, failure, cancellation, authorization,
  idempotency, recovery, and reconciliation paths proportionally to risk.
- Use deterministic fixtures. Avoid network, random IDs, and wall-clock timing
  in ordinary unit tests.

## Migrations

- SQLite migrations are embedded, forward-only in releases, and ordered.
- Every migration applies to an empty database and all maintained upgrade
  fixtures inside an explicit transaction when SQLite permits it.
- Destructive changes use expand/migrate/contract steps and documented backup
  or recovery behavior.
- Application code never silently edits schema outside the migration runner.
- Schema state is observable through `doctor` and support diagnostics.

## APIs and generated contracts

- REST JSON lives under `/api/v1`; live streams use versioned WebSocket paths.
- OpenAPI is the HTTP source of truth. Generated Go and TypeScript artifacts
  are isolated, deterministic, committed only when policy requires it, and
  never edited manually.
- Mutations accept idempotency keys and return durable operation IDs when work
  is asynchronous.
- Machine-readable CLI and MCP responses use stable, bounded, versioned
  schemas. Streaming output uses JSON Lines.
- Browser mutations require a same-origin local session and CSRF token.
- Unknown input fields fail validation unless a contract explicitly permits
  extension data.

## Time, cancellation, and state

- Store and compare time in UTC; localize only in presentation adapters.
- Propagate `context.Context` through I/O and long-running operations.
- Persist durable intent and outcomes, not volatile derived convenience state.
- State derived from live infrastructure includes evidence age and observer
  availability; unknown and stale are legitimate outcomes.
