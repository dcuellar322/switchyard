---
title: v1 compatibility and deprecation policy
description: Stability promises for manifests, APIs, CLI output, MCP, plugins, and local data.
category: reference
audience: [user, integrator, contributor]
since: 1.0.0
lastVerified: 2026-07-17
---

Switchyard follows semantic versioning for the product and publishes every
machine-consumed contract with an independent version marker. SQLite remains
an internal storage format and is not an integration API.

## Compatibility table

| Surface | v1 identifier | Compatibility promise |
|---|---|---|
| Product | `1.x.y` | Patch releases fix defects. Minor releases may add opt-in capabilities without breaking v1 workflows. |
| HTTP and WebSocket | `/api/v1`, `apiVersion: v1` | Existing methods, paths, required request fields, meanings, and error codes remain compatible for v1. Additive response fields are allowed. |
| CLI automation | `switchyard.cli/v1` | Commands, semantic exit classes, JSON/JSONL envelope fields, and published schemas remain compatible. Human text is not an automation contract. |
| Project manifest | `switchyard.dev/v1` | Unknown fields remain errors. Existing fields and defaults are not repurposed. Additions are optional. |
| MCP | stable tool names under server major `1` | Existing inputs, bounded outputs, risk annotations, and permission requirements are not weakened or repurposed. |
| Plugin JSON-RPC | `switchyard.plugin/v1` | Exact protocol negotiation. Additions must be safely ignorable or optional; capabilities and scopes retain their meaning. |
| Plugin manifest | `switchyard.plugin-manifest/v1` | Exact schema identifier, strict validation, and executable fingerprint review. |

The desktop bundles an exact-version sidecar. It may attach to another
Switchyard daemon with the same product major and `apiVersion: v1`; additive
database migrations do not make a compatible v1 daemon unusable by the shell.

## Deprecation

A stable capability is deprecated in release notes, reference documentation,
and the relevant CLI/UI discovery surface before removal. The normal support
window is at least two minor releases and 90 days, whichever is longer. A
security issue may require faster removal; that release must publish the risk,
replacement, and migration procedure.

Switchyard does not silently guess across major versions. Unsupported API,
plugin, manifest, or database versions fail before mutation with an actionable
error. A major-version migration must be previewable and create a verified
backup before changing durable data.

## Alpha and beta migration

Alpha/beta project manifests using `switchyard.dev/v1alpha1` are accepted and
normalized in memory. Preview or rewrite one explicitly with:

```bash
switchyard manifest migrate .switchyard/project.yml
switchyard manifest migrate .switchyard/project.yml --write
```

`--write` creates `.switchyard/project.yml.v1alpha1.bak` without overwriting an
existing backup, then atomically replaces the source. Alpha plugin executables
must be rebuilt against the v1 SDK and change both protocol identifiers; the
host never runs an incompatible plugin as a migration shortcut.

Embedded database migrations upgrade every schema produced by repository
alpha/beta builds. See [v1 data migration](migration-v1.md).
