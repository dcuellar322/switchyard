---
title: Public site Phase 6 evidence
description: Evidence for code-derived references, generated-doc drift gates, Markdown exports, llms files, and agent integration guidance.
category: contributor
audience: [maintainer, contributor]
lastVerified: 2026-07-17
---

## Outcome

Complete. The production Go binary and canonical schemas generate CLI,
manifest, settings, MCP, OpenAPI, and plugin references. All root docs are also
available as Markdown plus bounded `llms.txt` and `llms-full.txt` indexes.
Codex, Claude Code, and generic MCP pages lead with observe-only profiles,
project scope, authorization failures, destructive boundaries, and removal.

## Evidence

```bash
pnpm site:generate
git diff --exit-code -- docs/generated site/public/llms.txt site/public/llms-full.txt
```
