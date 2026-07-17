---
title: Durable local settings
description: Project roots, ports, retention, tools, AI providers, permissions, and appearance.
category: reference
audience: [user, integrator]
since: 1.0.0
lastVerified: 2026-07-17
---

Switchyard stores one revisioned settings document in its private SQLite
database. The browser, CLI, daemon, and MCP adapter read the same application
service; browser-local storage is not an authority for product configuration.
Each update is a full compare-and-swap replacement, so a stale browser or JSON
document receives `SETTINGS_REVISION_CONFLICT` instead of overwriting a newer
change. Audits record the actor, revision, and changed section names without
copying paths, provider configuration, or other values.

Open `/settings` from `switchyard ui` for the accessible editor. Headless users
can inspect or round-trip the same generated contract:

```bash
switchyard settings show
switchyard settings export ./switchyard-settings.json
# Review and edit the complete private file, preserving its revision.
switchyard settings apply ./switchyard-settings.json --yes
```

Export refuses to overwrite a file and creates mode `0600` output. Apply reads
at most 64 KiB, rejects unknown fields and trailing JSON, requires explicit
confirmation, and relies on the daemon for path canonicalization and all
cross-field validation.

## Effective behavior

| Section | Effect |
|---|---|
| Project roots | Applied immediately to new deterministic scans. Existing registered projects remain available. |
| Preferred ports and exclusions | Applied immediately to browser suggestions; the registry still reports every observed fact. |
| Terminal and editor | Applied immediately to built-in quick actions. Integrated terminal remains a typed authenticated session, not a shell string built by the browser. |
| Default agent profile | Applied to new `switchyard mcp serve` sessions unless an explicit `--profile` is supplied. |
| Appearance | Applied across browser routes. Density, timestamp preference, and high-contrast tokens remain client presentation only. |
| Retention | Applied on the next daemon start so collectors and pruning jobs switch bounds atomically. |
| AI provider adapter configuration | Applied on the next daemon start because provider processes are composed once. The selected default is live client preference. |

The settings response lists pending restart sections. A restart never pretends
to have happened: active collectors continue using the prior safe bounds until
the process starts again, at which point the pending list is empty.

## Project-root boundary

At first initialization, the user's home directory is the approved root. Add
narrower roots before removing it. Paths must exist, be directories, resolve
through symlinks, and cannot be a filesystem root. New scans outside the list
fail with `PROJECT_ROOT_DENIED` unless that individual request carries an
explicit override:

```bash
switchyard project add /opt/reviewed/repository --allow-outside-roots
```

The override does not trust the repository and does not execute its code. It
only permits the deterministic read-only discovery pass for that request. MCP
proposal creation never supplies the override.

## Credential references

No secret value belongs in the settings document. CLI providers store only an
executable and optional model. The OpenAI-compatible adapter accepts an HTTPS
endpoint, model, and an `env:NAME` reference. On daemon startup the reference
selects the environment variable; API responses, support bundles, audits, and
the database never contain its value. Choosing `none` keeps all onboarding and
diagnosis deterministic-only.

Daemon flags seed a new database. Once the settings singleton exists, its
restart-bound values win on later starts. Database backup, migration, and
restore procedures preserve the document and its revision with the rest of the
durable control-plane state.
