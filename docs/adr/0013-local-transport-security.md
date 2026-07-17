---
title: "ADR-0013: Local IPC and browser session security"
description: Protect privileged local clients and loopback browser mutations.
category: concept
audience: [contributor, integrator]
lastVerified: 2026-07-17
---

- Status: Accepted
- Date: 2026-07-15

## Context

CLI, desktop, MCP, and browser clients need local access to privileged
capabilities. Loopback binding alone does not prevent cross-origin requests or
access by other local users.

## Decision

Use user-permissioned Unix sockets on macOS/Linux and named pipes on Windows for
privileged local clients. Serve the browser API on loopback only. A local
bootstrap handshake issues a short-lived same-origin cookie; mutations require
CSRF validation. Validate origins for HTTP and WebSocket traffic, disallow
permissive CORS, expire idle sessions, and audit mutations.

## Consequences

Local clients share handlers while retaining transport-specific authentication.
Socket/pipe cleanup and permissions are tested. Browser JavaScript never
receives the Docker socket or generic process-execution capability.
