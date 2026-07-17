---
title: Public site Phase 0 evidence
description: Evidence for the public identity, canonical-domain contract, site boundary, brand inventory, and screenshot policy.
category: contributor
audience: [maintainer, contributor]
lastVerified: 2026-07-17
---

## Outcome

Repository work is complete. ADR-0017 accepts a separate static site, root
documentation ownership, GitHub Release authority, no analytics, and the
canonical `switchyard.davidcuellar.tech` origin. `docs/public-site.md` records
the differentiated name, audience, conversion goals, collision risk, tokens,
and generic screenshot inventory.

## Evidence

- `docs/adr/0017-static-public-site.md`
- `docs/public-site.md`
- `site/src/config/site.ts`
- deterministic images under `web/tests/visual/`

Cloudflare project ownership and DNS remain external launch-checklist items.
