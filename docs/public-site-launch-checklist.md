---
title: Public site launch checklist
description: Repository, release, Cloudflare, DNS, GitHub, security, accessibility, and production smoke gates for the canonical site.
category: contributor
audience: [maintainer]
since: 1.0.0
lastVerified: 2026-07-17
---

This checklist separates repository-complete work from external activation.
Never mark an operational item complete from local static output alone.

## Repository gates

- [x] Canonical identity and `SITE_URL` use
  `https://switchyard.davidcuellar.tech`.
- [x] Astro/Starlight output is static and contains no product API client or
  server runtime.
- [x] Root docs load through Starlight and expose Markdown equivalents,
  `llms.txt`, and `llms-full.txt`.
- [x] Required pages, redirects, security headers, robots, sitemap, RSS,
  structured data, unique metadata, and a default social card build locally.
- [x] Download URLs come only from normalized GitHub Release metadata; a
  published stable release with a missing artifact or checksum fails closed.
- [x] Package-manager methods stay hidden unless independently fetched package
  records match the stable release version, URLs, and checksums.
- [x] Chromium, Firefox, WebKit, mobile journeys, axe checks, and deterministic
  visual baselines are versioned.
- [x] Public screenshots come from generic deterministic fixtures and were
  reviewed for personal paths, private project names, tokens, and live data.
- [x] Privacy, security, contribution, support, code-of-conduct, license, and
  third-party attribution routes are linked.

Run:

```bash
make site-quality
make site-test-e2e
make site-test-visual
make repository-check
```

## Cloudflare and DNS activation

- [ ] Create or confirm the Cloudflare Pages project.
- [ ] Set `CLOUDFLARE_PROJECT_NAME` and the scoped account/token secrets in the
  `site-preview` and `production` GitHub environments.
- [ ] Attach `switchyard.davidcuellar.tech` to the Pages project and confirm
  domain ownership before changing DNS.
- [ ] Point the `switchyard` DNS name to Pages and wait for an active TLS
  certificate.
- [ ] Configure the production `pages.dev` hostname to redirect permanently,
  path-for-path, to the canonical origin.
- [ ] If `davidcuellar.tech/switchyard` is exposed, make it a permanent
  path-preserving redirect with no loop or duplicate content.
- [ ] Confirm preview deployments emit `X-Robots-Tag: noindex, nofollow` and
  production output does not.

## GitHub activation

- [ ] Set the repository description, homepage, topics, and social preview to
  the differentiated product identity.
- [ ] Enable Discussions and create question, ideas, integrations, and show-and-tell
  categories.
- [ ] Confirm `good first issue`, `help wanted`, security, documentation, bug,
  enhancement, and needs-triage labels exist.
- [ ] Enable private vulnerability reporting.
- [ ] Confirm branch protection requires public-site CI before deployment.

## Release and distribution activation

- [ ] Publish a reviewed stable GitHub Release with every required desktop and
  CLI artifact, checksum, platform signature, attestation, updater signature,
  and SBOM.
- [ ] Confirm the homepage, changelog, download choices, and RSS feed refresh
  from the published release without guessed filenames.
- [ ] Run clean-machine install, upgrade, verification, and uninstall smoke
  tests on supported macOS, Windows, Linux, and WSL paths.
- [ ] Generate Homebrew and WinGet drafts from the reviewed tag, submit them
  upstream, and merge only after fresh install and upgrade verification.
- [ ] Run the distribution monitor and confirm package methods appear only
  when published records match stable.

## Production review

- [ ] Run `pnpm --dir site smoke` against the canonical hostname.
- [ ] Confirm every primary route resolves once, keeps its path, and returns a
  canonical link on the approved origin.
- [ ] Validate the social card in GitHub and common link-preview debuggers.
- [ ] Run the production Playwright smoke suite on desktop and mobile.
- [ ] Confirm no serious or critical accessibility violations and review
  keyboard, focus, reduced motion, zoom, and screen-reader landmarks manually.
- [ ] Confirm download integrity links resolve and package versions match.
- [ ] Review legal copy, third-party notices, screenshot privacy, and every
  product claim against the released version.
