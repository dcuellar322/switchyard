# ADR-0002: Go as the local control-plane language

- Status: Accepted
- Date: 2026-07-15

## Context

The control plane needs portable binaries, concurrency, cancellation, local
systems integration, and predictable idle overhead.

## Decision

Implement domain behavior, application services, lifecycle coordination,
persistence, APIs, CLI, and MCP in current stable Go pinned by the repository.
Use explicit typed code and standard-library capabilities where practical.

## Consequences

One toolchain owns operational behavior and cross-platform adapters can share
contracts. Vue and Tauri remain clients; Rust or TypeScript business logic
cannot become an alternate control plane.
