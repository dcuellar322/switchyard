---
title: "ADR-0010: MCP-first provider-neutral agent integration"
description: Give coding agents bounded project context and operations through shared use cases.
category: concept
audience: [integrator, contributor]
lastVerified: 2026-07-17
---

- Status: Accepted
- Date: 2026-07-15

## Context

Coding agents need stable project context and safe operations without guessing
shell commands or coupling Switchyard to one provider.

## Decision

Expose bounded typed resources, prompts, and tools through MCP over shared
application use cases. Keep provider adapters under the agents context behind
provider-neutral interfaces. Core operation remains deterministic without AI.
No generic unrestricted shell tool is part of the MCP surface.

## Consequences

Codex, Claude Code, and other clients receive the same operational vocabulary.
Agent-originated mutations require explicit permissions and audit. Provider
output is untrusted, schema-constrained input requiring validation and human
approval.
