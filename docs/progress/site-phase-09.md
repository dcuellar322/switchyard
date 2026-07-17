---
title: Public site Phase 9 evidence
description: Evidence for reviewed Homebrew and WinGet drafts, status monitoring, version gates, and hidden unavailable methods.
category: contributor
audience: [maintainer, contributor]
lastVerified: 2026-07-17
---

## Outcome

Guarded automation is complete. A reviewed stable tag generates Homebrew and
WinGet drafts only from normalized release URLs and checksums. The workflow
requires the release tag to match stable and uploads drafts for maintainer
review. Monitoring marks a channel available only when the upstream package
record contains the same version, URLs, and digests.

No stable release or upstream package currently exists, so the website hides
both methods and direct signed downloads remain the future fallback. Fresh
install, upgrade, and publication cannot be verified before those external
records exist.
