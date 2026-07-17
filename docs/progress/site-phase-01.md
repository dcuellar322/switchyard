---
title: Public site Phase 1 evidence
description: Evidence for the Astro and Starlight foundation, root docs loader, scripts, CI, preview policy, and canonical output.
category: contributor
audience: [maintainer, contributor]
lastVerified: 2026-07-17
---

## Outcome

Repository work is complete. `site/` is a strict pnpm workspace package with
Astro, Starlight, shared tokens, layouts, navigation, 404, robots, sitemap,
social metadata, and Cloudflare-compatible output. CI builds previews with
`noindex`; deployment requires the documented environment secrets.

## Evidence

```bash
pnpm site:check
pnpm site:build
pnpm site:validate
```

The production build contains more than 120 pages, canonical URLs on the approved
origin, a Pagefind search index, RSS, and sitemap output.
