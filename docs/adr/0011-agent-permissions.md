# ADR-0011: Agent permission profiles and no generic shell tool

- Status: Accepted
- Date: 2026-07-15

## Context

Agent clients need useful project operations, but prompt content and model
output cannot be trusted as an authorization boundary.

## Decision

Enforce application-level `observe`, `develop`, `maintain`, and `admin`
profiles scoped by provider and project. Default to observe. Annotate MCP tools
with accurate risk/idempotency metadata, audit every agent mutation, and keep
destructive tools disabled by default. Never expose a generic unrestricted
shell MCP tool; agents may invoke only approved typed project actions.

## Consequences

Permissions remain effective independent of prompts or client behavior.
Providers receive less ambient power and users can review agent activity.
Adding a tool requires a use case, risk classification, bounded contract, and
permission tests.
