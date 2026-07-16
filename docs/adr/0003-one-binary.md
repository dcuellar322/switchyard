# ADR-0003: One Switchyard binary with daemon, CLI, and MCP subcommands

- Status: Accepted
- Date: 2026-07-15

## Context

Separate executables would complicate versioning, installation, local protocol
compatibility, and desktop sidecar packaging.

## Decision

Ship one primary executable named `switchyard`. `cmd/switchyard` is a small
composition root; command groups delegate to client or application packages.
The binary exposes daemon, CLI, UI, doctor, and MCP entry points without
centralizing their behavior in one package.

## Consequences

Users install and update one version. Internal packages must remain cohesive so
the deployment unit does not become a god binary. The desktop app bundles or
attaches to a compatible copy of the same executable.
