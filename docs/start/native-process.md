---
title: Start your first native-process project
description: Review an argument-array command, start its process tree, follow output, and stop without orphans.
category: tutorial
audience: [user]
platforms: [macos, linux, windows, wsl]
since: 1.0.0
lastVerified: 2026-07-17
sidebar:
  order: 4
---

Native projects can use uv, npm, pnpm, Make, or another explicit executable.
Switchyard does not require Docker for this path.

## 1. Review discovery

```bash
switchyard project add /absolute/path/to/native-project
switchyard manifest explain <project>
```

Confirm the executable, argument array, working directory, environment
references, dependency order, expected ports, and health checks. Shell syntax
is rejected unless the manifest explicitly enables it and surfaces the risk.

## 2. Preview and start

```bash
switchyard plan start <project>
switchyard start <project>
switchyard status <project>
```

Success is a durable run with a process-identity fingerprint and a running or
healthy project state. A stored PID alone is never sufficient evidence.

## 3. Read output and stop

```bash
switchyard logs <project> --tail 50
switchyard stop <project>
switchyard status <project>
```

Switchyard captures stdout and stderr separately, sends graceful termination
to the process group or Job Object, and escalates only after the configured
timeout. Success ends with no managed child process left behind.
