---
title: Out-of-process plugins
description: Versioned protocol, trust, capabilities, supervision, and conformance boundaries.
category: concept
audience: [integrator, contributor]
since: 1.0.0
lastVerified: 2026-07-17
---

Phase 16 applies ADR-0012 with a public Go SDK under `sdk/plugin` and an exact,
versioned JSON-RPC 2.0 protocol over stdin/stdout. A plugin is always a separate
supervised process. The daemon never loads a shared library, calls a shell, or
lets plugin-specific types enter a core product domain.

## Discovery and trust

Switchyard deterministically scans `<data-dir>/plugins/<package>/plugin.json`.
Discovery reads a bounded manifest and hashes that manifest together with a
bounded regular executable file. It does not execute repository files or the
plugin. Executables must remain inside their package, cannot be symlinks, and
cannot be group- or world-writable.

The UI and CLI show the SHA-256 package fingerprint, protocol, capabilities,
and requested scopes. Trust records the exact reviewed fingerprint but leaves
the plugin disabled. Enabling requires an explicit subset of requested scopes
and a successful initialize/health exchange. Any manifest or executable change
changes the fingerprint, disables the plugin, revokes grants, and requires a
new review.

## Process and protocol boundary

Every method call starts a new bounded process in the plugin package directory
with an argument array and a minimal environment containing no provider keys or
repository secrets. Switchyard negotiates `switchyard.plugin/v1`, then
verifies that the running ID, version, capabilities, requested scopes, and
grants exactly match the trusted package. Messages are capped at 1 MiB and
unknown response fields are rejected.

The stable method set is:

- `initialize` for exact protocol and scope negotiation;
- `plugin.health` for liveness without ambient project data;
- `project.inspect` for structured facts and advertised actions; and
- `project.operate` for one typed action through a durable host operation.

There is no plugin-to-host callback channel and no generic shell method. A
project root is omitted unless `project.files.read` was explicitly granted.
Operations require `project.operate`, recheck project trust at execution time,
and use the existing operation coordinator for cancellation and audit.

Plugin stderr is bounded, redacted, retained to the newest 1,000 entries, and
never parsed as protocol. A crash, timeout, malformed response, oversized
message, or identity mismatch fails only the call, records unhealthy state, and
cannot unwind the daemon. On Unix, timed-out process groups are terminated so
children do not outlive supervision.

## OS trust boundary

Plugins run with the installing local user's operating-system identity.
Switchyard strips inherited secrets and mediates Switchyard-owned data and
mutations, but it is not an OS sandbox. Trusting a plugin therefore means
trusting locally installed executable code, not just approving API scopes. The
review UI states this distinction explicitly.
