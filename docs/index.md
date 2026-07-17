---
title: Switchyard documentation
description: Install Switchyard, bring a project online, connect coding agents, troubleshoot safely, or inspect the complete reference.
category: tutorial
audience: [user, contributor, integrator]
since: 1.0.0
lastVerified: 2026-07-17
slug: docs
template: splash
hero:
  tagline: Task-oriented guides for the local development command center.
  actions:
    - text: Start in five minutes
      link: /docs/start/
      icon: right-arrow
      variant: primary
    - text: Download Switchyard
      link: /download/
      icon: download
---

## Start here

- [Choose an installation](/docs/start/install/) for macOS, Windows, Linux,
  WSL2, or headless use.
- [Add your first project](/docs/getting-started/) through deterministic
  discovery and explicit trust.
- [Run a Compose project](/docs/start/compose/) or
  [a native-process project](/docs/start/native-process/).
- [Follow logs and health](/docs/start/logs-health/) and
  [resolve a port conflict](/docs/start/port-conflict/).
- [Coordinate a workspace](/docs/start/workspace/).

## Operate and integrate

- **Projects and manifests:** [onboarding concepts](/docs/architecture/project-onboarding/)
  and [manifest reference](/docs/manifest-reference/)
- **Runtime:** [Docker Compose](/docs/architecture/docker-compose-runtime/),
  [native processes](/docs/architecture/native-process-runtime/), and
  [terminal sessions](/docs/architecture/terminal-sessions/)
- **Evidence:** [logs, health, and resources](/docs/architecture/observability/)
  and [support bundles](/docs/support-bundles/)
- **Agents:** [Codex guide](/integrations/codex/),
  [Claude Code guide](/integrations/claude-code/), and
  [MCP reference](/docs/mcp/)
- **Automation:** [CLI contract](/docs/cli/) and
  [plugin SDK](/docs/plugin-sdk/)

## Troubleshoot and contribute

Start with [troubleshooting by symptom](/docs/troubleshooting/). It separates
safe evidence from sensitive source, environment, or diagnostic data.

Contributors should read the [architecture overview](/docs/architecture/),
[engineering conventions](/docs/conventions/), and
[adapter development guide](/docs/adapter-development/) before changing a
domain or platform boundary.
