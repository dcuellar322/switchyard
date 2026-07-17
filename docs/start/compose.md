---
title: Start your first Docker Compose project
description: Review a discovered Compose topology, preview lifecycle, start it, and verify service health.
category: tutorial
audience: [user]
platforms: [macos, linux, windows, wsl]
since: 1.0.0
lastVerified: 2026-07-17
sidebar:
  order: 3
---

This tutorial assumes `docker version` and `docker compose version` work in the
same user context as Switchyard.

## 1. Add the repository

```bash
switchyard project add /absolute/path/to/compose-project
switchyard project list
```

Open the proposal in the browser. Confirm the Compose files, project identity,
services, published ports, health checks, and source evidence before trust.
Discovery reads configuration but does not run Compose.

## 2. Preview the start plan

```bash
switchyard plan start <project>
```

The plan should name the installed `docker compose` lifecycle, selected files,
working directory, and affected services. No container starts during preview.

## 3. Start and observe

```bash
switchyard start <project>
switchyard status <project>
switchyard logs <project> --tail 50
```

Success is an operation in a terminal state, declared services mapped through
Compose labels, and required health checks reporting healthy. A running
container with a failed health check is **degraded**, not falsely stopped.

## What happened?

Switchyard executed lifecycle through the installed Compose CLI and observed
containers, events, logs, published ports, and metrics through the Docker API.
Stopping preserves containers and data; teardown remains a separately
previewed destructive operation.
