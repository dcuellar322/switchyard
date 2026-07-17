---
title: Public site Phase 3 evidence
description: Evidence for release normalization, artifact integrity, supported platform guides, unavailable states, and rebuild triggers.
category: contributor
audience: [maintainer, contributor]
lastVerified: 2026-07-17
---

## Outcome

The implementation is complete and fails closed. GitHub API metadata is the
only binary URL source. The stable artifact contract requires the supported
desktop and CLI matrix plus checksums. The UI keeps every platform visible and
shows an honest unavailable state because the repository has no published
release at this verification date.

## Evidence

```bash
pnpm site:test
pnpm site:build
pnpm site:test:e2e
```

Release publication triggers the deploy and package-draft workflows. A real
install-path smoke remains pending the first reviewed stable release. The
release workflow uses the same tested artifact classifier, builds macOS arm64
and Intel desktop packages separately, and refuses to draft a stable release
until the complete matrix has aggregate checksums.
