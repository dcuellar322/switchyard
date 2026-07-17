---
title: Follow logs and verify project health
description: Separate process state from readiness and collect bounded redacted evidence.
category: tutorial
audience: [user]
platforms: [macos, linux, windows, wsl]
since: 1.0.0
lastVerified: 2026-07-17
sidebar:
  order: 5
---

## Read a bounded snapshot

```bash
switchyard status <project>
switchyard logs <project> --tail 100
```

Status reports runtime observation, health, stale/disconnected evidence, and
project derivation separately. Logs keep original messages while adding
project, service, run, source, stream, timestamp, and best-effort level fields.

## Follow in the browser

```bash
switchyard ui --path /projects/<project-id>
```

Open **Logs** to follow the redacted stream and **Overview** for required health
checks. Reconnection uses bounded sequence replay and then refreshes state when
the replay window is insufficient.

## Observable success

A healthy project has every required check passing. A live process with a
failed check remains running and becomes degraded. When an observer is
unavailable or data is old, Switchyard shows disconnected or stale instead of
inventing readiness.

For shareable evidence, preview the support bundle before writing it:

```bash
switchyard doctor --bundle --preview
```
