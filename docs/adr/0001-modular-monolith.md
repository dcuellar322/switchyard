---
title: "ADR-0001: Modular monolith and domain dependency direction"
description: Keep Switchyard cohesive while enforcing bounded-context dependency rules.
category: concept
audience: [contributor, maintainer]
lastVerified: 2026-07-17
---

- Status: Accepted
- Date: 2026-07-15

## Context

Switchyard coordinates many local capabilities but ships and evolves as one
coherent product. Premature services would add deployment and consistency costs
without independent scaling or ownership needs.

## Decision

Build one modular Go monolith grouped by bounded context. Dependencies flow
from transports/adapters to application use cases to domain models.
Infrastructure implements ports owned by consuming application packages.
Domains do not read each other's tables and collaborate only through explicit
interfaces or typed events. Composition uses manual constructor injection.

## Consequences

The product remains easy to build and operate while package boundaries stay
enforceable. Cross-domain features require explicit contracts. Architecture
checks will reject forbidden imports and miscellaneous shared packages.
