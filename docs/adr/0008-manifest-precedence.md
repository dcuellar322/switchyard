---
title: "ADR-0008: Project manifest precedence and provenance"
description: Resolve effective project configuration without losing source evidence.
category: concept
audience: [contributor, integrator]
lastVerified: 2026-07-17
---

- Status: Accepted
- Date: 2026-07-15

## Context

Projects need portable definitions, machine-local values, generated inference,
live discoveries, and one-operation overrides without silently losing the
origin of effective configuration.

## Decision

Resolve each field using, highest first: runtime override,
`.switchyard/project.local.yml`, `.switchyard/project.yml`, accepted generated
inference, and live deterministic discovery. Retain field-level provenance and
confidence. Unknown fields fail validation. Local overlays never rewrite the
portable manifest.

## Consequences

Users can explain and diff effective values. Merge logic must operate on typed
models rather than generic maps. Schema versions and migrations are explicit,
and proposals remain reviewable until accepted.
