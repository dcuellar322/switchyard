# ADR-0015: Platform support order

- Status: Accepted
- Date: 2026-07-15

## Context

Process, port, IPC, PTY, notification, launcher, and packaging behavior differs
substantially by operating system. Attempting simultaneous parity would slow
the product and spread conditionals through domains.

## Decision

Implement and validate macOS first, Linux second, and Windows/WSL third.
Platform-specific behavior lives behind capability interfaces and OS adapter
packages. Domain and application packages remain platform-neutral.

## Consequences

Early releases may have explicitly documented platform gaps. Cross-platform
contracts are designed early but adapters are implemented in roadmap order.
Unsupported capability is a typed, explainable state rather than silent
failure.
