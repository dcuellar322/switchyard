---
title: Troubleshooting by symptom
description: Safe checks and bounded recovery steps for common Switchyard runtime and platform failures.
category: troubleshooting
audience: [user, contributor]
platforms: [macos, linux, windows, wsl]
since: 1.0.0
lastVerified: 2026-07-17
searchTerms: [Docker unavailable, port in use, logs unavailable, daemon disconnected, WSL localhost]
---

Start with `switchyard doctor`, then `switchyard debug logs --level warn` and
`switchyard data inspect`. Preview `switchyard doctor --bundle --preview`
before writing or sharing an archive. Keep diagnostic output private even
after automated redaction.

- **Daemon will not start:** another matching daemon may own `daemon.lock` or
  the Unix socket/named pipe. Do not delete a live lock. Stop the process shown
  in the lock or let normal stale-lock recovery run.
- **Docker disconnected:** verify `docker version`, `docker compose version`,
  and the selected Docker context as the same user running Switchyard.
- **Terminal unavailable:** Linux needs a PTY-capable supported distribution;
  external launch also needs a supported terminal emulator. Windows requires
  a supported ConPTY host and Windows Terminal for external handoff.
- **Ports unavailable:** host policy may prevent process ownership details.
  Bindings without an accessible PID remain honest partial observations.
- **Manifest rejected:** unknown fields and unsupported versions fail closed.
  Run `switchyard manifest migrate <path>` for alpha manifests, then validate.
- **Database newer than binary:** install the matching/newer Switchyard build or
  restore a pre-migration backup to a separate data directory. Never force an
  old binary to open it.
- **Plugin disabled after update:** executable or manifest fingerprints changed.
  Refresh, inspect the new capabilities/scopes, then explicitly trust again.
- **WSL attach fails:** Windows named pipes and WSL Unix sockets are separate.
  Run the Linux binary inside WSL and use its `switchyard ui` URL.

Issue reports should include platform, Switchyard version, expected/actual
behavior, minimal reproduction, and only redacted diagnostic facts. Never
attach credentials, private source, browser bootstrap URLs, raw environment
files, or unreviewed application logs. The
[support-bundle contract](support-bundles.md) documents the exact archive
contents and residual privacy risk.
