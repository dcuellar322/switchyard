---
title: Native desktop shell
description: Thin Tauri adapter, sidecar lifecycle, tray, updater, and deep-link boundaries.
category: concept
audience: [user, contributor]
platforms: [macos, linux, windows]
since: 1.0.0
lastVerified: 2026-07-17
---

Phase 15 packages Switchyard as a Tauri 2 application while preserving
ADR-0009: Rust is a native adapter, not a second control plane. The bundled Go
sidecar still owns daemon startup, local IPC, authentication, projects,
workspaces, operations, resources, persistence, and every mutation policy.

## Startup and compatibility

The native process executes a fixed `switchyard version --json` command and a
read-only `switchyard desktop snapshot --json` command. The latter attaches to
a healthy daemon or lets the bundled binary safely start its detached daemon.
Before requesting a browser credential or exposing a project/workspace action,
the shell requires:

- an exact desktop and bundled-sidecar semantic-version match;
- a running daemon in the same product major version;
- `switchyard.api/v1`; and
- SQLite schema version 13 or newer.

The minimum schema is the first v1 desktop snapshot contract. Later v1 schema
additions are compatible because the shell consumes only the versioned bounded
snapshot and browser URL; it does not read SQLite or infer capabilities from
table versions.

Incompatibility is shown as a startup failure and no native mutation is
attempted. SQLite independently refuses to open a database whose applied
migration is newer than the binary's embedded migration set. That second gate
protects command-line and rollback launches even when no desktop process is
present.

After preflight, `switchyard ui --path ... --json` issues a short-lived browser
bootstrap credential. The webview navigates only to the returned HTTP
loopback URL. The startup document has only `core:default`; the remote
loopback-origin Vue application has no Tauri capability entry. It therefore
cannot invoke the shell plugin, arbitrary processes, updater, autostart,
notification, or deep-link APIs from JavaScript.

## Native responsibilities

Rust owns only OS-facing presentation:

- a dynamically refreshed tray showing daemon state, up to eight recent
  projects and workspaces, and fixed open/start/stop actions;
- a persisted close preference, defaulting to hide-to-tray;
- opt-in launch-at-login through the platform-native Tauri autostart adapter;
- single-instance activation and bounded `switchyard://project/<id>` and
  `switchyard://workspace/<id>` deep links;
- native notifications for new failed/partial operations, health transitions,
  daemon disconnect/recovery, Docker disconnect, port conflicts, and sustained
  90% host CPU or memory observations; and
- signed release update checks initiated from the tray.

The first snapshot initializes notification state without replaying historical
failures. Active host warnings notify once until they clear; disconnect and
health notifications are transition-based. Sidecar output is schema-checked
and capped at 1 MiB. Failure notifications never include raw sidecar stderr or
repository/log contents.

Tray mutations map to fixed argument arrays. Resource identifiers accept only
bounded ASCII alphanumeric, hyphen, and underscore characters. There is no
generic command, shell, Docker, SQL, or filesystem invocation surface in the
desktop adapter.

## Lifetime and updates

The daemon is independent of the window and tray. With the default preference,
closing the window hides it; selecting Quit exits only the native adapter so
CLI and browser clients remain available. Disabling the preference makes a
window close exit the adapter.

Debug builds do not load or expose the updater. Release compilation requires a
public Minisign key and HTTPS update endpoint. The release workflow also
requires the corresponding private updater key plus Apple Developer ID and
notarization credentials. Tauri verifies the downloaded signature before
installation; the shell restarts only after successful installation. Release
artifacts are drafted rather than published automatically so maintainers can
review signatures, notarization, changelog, and compatibility.

Desktop support followed the ADR-0015 order: macOS first, followed by the
Phase 18 Linux and Windows bundles. Interactive Windows sessions use the native
ConPTY adapter; WSL remains a separate Linux-daemon boundary as documented in
the platform support matrix.
