---
title: "ADR-0014: Bounded log retention and redaction"
description: Keep log memory and disk bounded while applying one redaction policy to every sink.
category: concept
audience: [contributor, integrator]
lastVerified: 2026-07-17
---

- Status: Accepted
- Date: 2026-07-15

## Context

Runtime logs are high-volume, may contain credentials, and are consumed live,
persisted, exported, searched, and sent in optional diagnostics or provider
evidence.

## Decision

Keep a bounded in-memory ring per active service and rotating NDJSON files per
run. Store segment identity, time ranges, checksums, and optional indexes in
SQLite. Apply the same configurable redaction pipeline before display,
persistence, export, diagnostics, or AI use. Enforce retention by age and disk
cap and indicate that a value was redacted without exposing it.

## Consequences

Log memory and disk use stay predictable. Redaction correctness is a security
quality gate across every sink. Full-text indexing and compression may be added
without changing the canonical event model.
