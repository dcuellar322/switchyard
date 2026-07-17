---
title: Choose and verify an installation
description: Select desktop or headless use, confirm platform requirements, and verify the authoritative release asset.
category: tutorial
audience: [user]
platforms: [macos, linux, windows, wsl]
since: 1.0.0
lastVerified: 2026-07-17
sidebar:
  order: 1
---

## 1. Choose the operating mode

Use the **desktop command center** for the native window, tray, notifications,
autostart, and signed updater. Use the **headless CLI** for SSH hosts, CI,
WSL2, or a browser-only workflow.

The [download page](/download/) shows only assets returned by the GitHub
Releases API. If a verified stable package is unavailable, it says so instead
of guessing a filename.

## 2. Verify before installation

From the same release, verify the asset checksum, platform identity, GitHub
attestation, and SBOM as applicable. Follow the
[verification guide](/download/verify/) for exact commands.

## 3. Confirm local capabilities

```bash
switchyard doctor
```

Expected success is a versioned report with local IPC, database/schema, Git,
and the adapters available on this machine. Docker may be unavailable when the
first project uses only native processes.

## 4. Open the product

```bash
switchyard ui
```

The command prints a short-lived authenticated loopback URL. Do not paste that
URL into an issue or support bundle. The browser should show the empty project
state and remain usable when Docker is disconnected.

Next: [add your first project](../getting-started.md).
