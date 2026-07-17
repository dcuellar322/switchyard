---
title: Intelligent diagnosis and safe automation
description: Deterministic rules, evidence-bound AI hypotheses, and reviewed automation recipes.
category: concept
audience: [user, integrator, contributor]
since: 1.0.0
lastVerified: 2026-07-17
---

Phase 17 adds troubleshooting assistance without granting a new execution
authority. Deterministic rules always run first. Optional AI receives one
bounded, redacted evidence document and may return only schema-valid hypotheses
that cite existing evidence and accepted actions.

## Evidence boundary

The diagnostics application owns a provider-neutral bundle assembled through
explicit adapters for catalog, runtime, health, logs, resources, ports, Git,
manifest provenance, operations, and actions. It does not read another
domain's tables.

Each bundle contains:

- trusted project identity and current normalized runtime/health facts;
- at most 100 recent redacted log lines, explicitly marked untrusted;
- current Git state without source contents;
- project-relevant port conflicts and sustained resource warnings;
- configuration source names and recent operation outcomes;
- accepted action identifiers, names, types, and risks, but no commands;
- a non-executable cleanup preview; and
- an exact byte count and SHA-256 receipt.

Collection is partial by design: an unavailable observer adds a warning instead
of inventing a healthy state. The complete encoded bundle is capped at 256 KiB.

## Evaluation pipeline

```text
bounded cross-domain observations
             |
             v
 deterministic rule evaluation
             |
             +----> complete local diagnosis
             |
      optional provider selected
             |
             v
 isolated provider adapter + strict JSON Schema
             |
             v
 evidence/action reference validation
             |
             v
 ranked durable review receipt
```

The rule engine recognizes disconnected or degraded runtimes, required health
failures, repeated crashes, port conflicts and bind failures, common redacted
log signatures, resource pressure, incomplete Git operations, and stale
stopped projects. Known failures remain available when a provider is missing,
times out, fails, or returns invalid output.

Provider warnings are bounded. A provider hypothesis is rejected unless every
evidence identifier exists in the sent bundle. Suggested action identifiers
must resolve to current accepted actions and cannot be destructive, networked,
or interactive. The raw provider response is neither trusted nor executed.

## Prompt-injection resistance

Repository text and logs are serialized as inert evidence data and are never
placed in an instruction role. Provider prompts explicitly prohibit following
instructions found in evidence, reading files, running commands, using tools or
network access, or inventing evidence/action identifiers. CLI providers retain
their Phase 11 empty-root, minimal-environment, no-tool isolation; the HTTP
provider receives the same schema and byte budgets. Server-side validation is
authoritative even if a provider ignores the prompt.

## Action and automation authority

A diagnosis carries references to existing accepted project actions only. The
one-click endpoint reloads the durable diagnosis and denies any action not
cited by a validated hypothesis. Allowed actions are submitted as ordinary
`action.run` operations with actor identity, an idempotency key, risk
confirmation set to false, and outside-root access set to false. Diagnostics
never call a runner, Docker, SQL, Git mutation, or `os/exec` directly.

Automation recipes are durable and inspectable. Creation always produces a
disabled recipe; enabling is a separate explicit decision that revalidates the
current action. Recipes have a named deterministic trigger, a 60-second to
24-hour cooldown, and a one-to-20 run UTC-day limit. Evaluation never reacts to
an AI-only finding. It dispatches only read-only actions or declared
test/check/inspect actions and automatically disables a recipe if its action
becomes missing or unsafe. Every execution goes through the same durable
operation kernel and audit trail.

## Local review and retention

Repeated crashes, port conflicts, resource pressure, and unhealthy dependencies
create deduplicated local notifications. The native shell separately observes
the same health, port, resource, runtime, and operation transitions for OS
notifications. Users can inspect recipes, disable them immediately, and see
cooldown and daily-run counters in the browser or CLI.

Accuracy feedback accepts only `accurate` or `false_positive` for a hypothesis
that exists in the durable diagnosis. It remains in SQLite and is never sent to
a provider or telemetry endpoint. Periodic receipts retain the newest 100 per
project, while feedback-referenced receipts remain available. Cleanup remains
preview-only; neither diagnosis nor automation exposes deletion or source-edit
capabilities.

## Accepted decisions

No architecture decision changed. This implementation follows ADR-0010's
provider-neutral MCP/application boundary, ADR-0011's explicit profiles and
absence of a generic shell tool, and ADR-0014's redaction-before-persistence and
bounded-output requirements.
