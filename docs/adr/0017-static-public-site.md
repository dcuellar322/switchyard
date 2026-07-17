---
title: "ADR-0017: Static public site and documentation portal"
description: Keep the public website static, bounded, canonical, and backed by root documentation.
category: concept
audience: [contributor, maintainer]
lastVerified: 2026-07-17
---

- Status: Accepted
- Date: 2026-07-17

## Context

Switchyard needs a public product site, documentation portal, download
experience, and contribution funnel. The local Vue application is an
authenticated adapter over privileged daemon capabilities and has a different
deployment, navigation, content, and trust lifecycle from a public website.
The root `docs/` tree already owns product documentation and must not gain a
divergent website copy.

## Decision

Add a separate `site/` pnpm workspace package built with Astro, Starlight, and
strict TypeScript. It produces portable static output with no backend, login,
or runtime database. Starlight loads Markdown directly from the repository
root `docs/` directory. Marketing, releases, documentation, community, SEO,
and design-system code remain explicit site domains; site code may reuse
approved generated tokens and sanitized product screenshots but may not import
Vue stores, generated API clients, or application domain services.

The sole production origin is
`https://switchyard.davidcuellar.tech`. Canonical URLs, sitemap entries, feeds,
structured data, and social metadata derive from one typed configuration.
Cloudflare Pages hosts the static output. Preview output is non-indexable, and
the generated Pages production hostname must redirect path-for-path to the
canonical origin through host configuration.

GitHub Releases remains authoritative for binaries. A build-time adapter
normalizes only API-provided asset URLs and fails closed when a published
stable release does not satisfy the versioned release-artifact contract. The
site never proxies binaries or guesses a filename.

The site launches without behavioral analytics. Adding analytics requires a
new explicit data and retention decision.

## Consequences

The public site can be deployed independently without expanding the Go daemon's
attack surface. Documentation remains reviewable beside implementation and can
serve both human and agent-readable forms. Preview deployments and DNS/domain
attachment still require external Cloudflare and GitHub repository settings;
repository progress reports must distinguish those operational checks from
locally verified static output.
