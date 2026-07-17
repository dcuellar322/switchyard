---
title: "ADR-0009: Tauri as a thin desktop shell"
description: Keep native integration in Tauri and product policy in the Go control plane.
category: concept
audience: [contributor]
lastVerified: 2026-07-17
---

- Status: Accepted
- Date: 2026-07-15

## Context

Switchyard needs a small native desktop experience, tray integration,
notifications, autostart, deep links, and signed updates without duplicating
the control plane.

## Decision

Use Tauri 2 with minimal Rust glue. Bundle the Go binary as a sidecar or attach
to a compatible daemon. All project, runtime, security, and persistence policy
remains in Go application services.

## Consequences

Browser, desktop, CLI, and agents share behavior. Native features are adapters,
not an alternate product core. Sidecar compatibility must be checked before a
desktop client mutates state.
