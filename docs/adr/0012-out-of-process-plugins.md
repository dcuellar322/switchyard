---
title: "ADR-0012: Versioned out-of-process plugin protocol"
description: Extend Switchyard through supervised capability-scoped processes.
category: concept
audience: [integrator, contributor]
lastVerified: 2026-07-17
---

- Status: Accepted
- Date: 2026-07-15

## Context

Additional runtimes and tools should be extensible without loading untrusted
code into the daemon or coupling core domains to plugin implementations.

## Decision

Plugins run as supervised external processes using a versioned JSON-RPC
protocol. Each declares executable identity, protocol version, capabilities,
and requested scopes. Users explicitly trust and enable plugins. Core domains
depend only on stable application contracts and never import plugin-specific
code.

## Consequences

A plugin crash cannot directly crash the daemon. Serialization, supervision,
capability enforcement, health, compatibility, and conformance testing become
part of the SDK. Shared-library loading and an uncurated automatic marketplace
are out of scope for the core protocol.
