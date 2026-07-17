---
title: "ADR-0004: REST, WebSocket, and OpenAPI transport contracts"
description: Keep browser and client contracts versioned, generated, and inspectable.
category: concept
audience: [contributor, integrator, maintainer]
lastVerified: 2026-07-17
---

- Status: Accepted
- Date: 2026-07-15

## Context

Browser, CLI, desktop, plugin, and agent clients need inspectable contracts,
while logs, events, terminals, and sessions need streaming behavior.

## Decision

Use REST JSON under `/api/v1` for commands and queries and versioned WebSockets
for live streams. OpenAPI is the HTTP source of truth and generates Go transport
types and TypeScript clients. Errors use RFC 9457-style problem details.

## Consequences

Contracts remain browser-friendly and debuggable. Generated code must be
isolated and reproducible. Transport handlers translate requests and responses
only; they cannot own orchestration or infrastructure access.
