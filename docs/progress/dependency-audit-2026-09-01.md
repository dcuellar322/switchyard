---
title: "Dependency audit: 2026-09-01"
description: Evidence for the September 2026 repository-wide dependency audit and upgrade.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-09-01
---

## Scope and release-age policy

This maintenance slice audited every dependency surface in the repository:
Go modules and build tools, the root/web/site pnpm workspace, Rust crates, pinned
language toolchains, GitHub Actions, dependency-bearing test fixtures, and
container image tags.

The eligibility cutoff was `2026-08-22T23:59:59Z`. A selected release therefore
had to be published no later than August 22, giving every selected version at
least ten full calendar days of public availability on September 1. No critical
finding required an age-policy exception.

The pnpm workspace now enforces the policy for direct and transitive packages
with `minimumReleaseAge: 14400`, strict release-time handling, and no allowance
for missing publication timestamps. GitHub Actions are pinned to immutable
commit SHAs with readable release comments.

## Upgrades applied

- Toolchains: Go `1.26.5` to `1.27.0`, Node `26.5.0` to `26.7.0`, pnpm
  `11.13.1` to `11.22.0`, and Rust `1.97.1` to `1.98.0`.
- Go build tools: oapi-codegen `2.7.2` to `2.8.0`, govulncheck `1.6.0` to
  `1.7.0`, and GolangCI-Lint `2.11.2` to `2.13.0`. sqlc `1.31.1` was already
  current and eligible. The oapi-codegen configuration preserves the existing
  exported enum names and route-registration order while accepting the new
  generated client validation.
- Go modules: all eligible direct and loaded transitive updates were resolved.
  Direct changes include chi `5.3.2`, oapi-codegen/runtime `1.7.0`, and
  modernc SQLite `1.57.0`; the complete resolved graph is recorded in
  `go.mod` and `go.sum`.
- JavaScript: every eligible compatible direct and transitive package was
  updated. Direct changes include Astro `7.2.4`, Vite `8.2.2`, Vitest `4.1.11`,
  ESLint `10.9.0`, typescript-eslint `8.67.0`, Wrangler `4.125.0`,
  `@tanstack/vue-query` `5.102.0`, `@lucide/vue` `1.33.0`, and
  `@testing-library/jest-dom` `7.0.1`. Security overrides move fast-uri to
  `3.1.5` and js-yaml to `4.3.1`.
- Rust: serde `1.0.229` and serde_json `1.0.151` were updated directly, and the
  entire eligible Cargo graph was re-resolved. Direct Tauri packages were
  already at their newest eligible releases.
- CI supply chain: all workflow actions were upgraded to their newest eligible
  releases and pinned by immutable SHA, including checkout/setup actions,
  CodeQL, GolangCI-Lint, Gitleaks, dependency review, RustSec, release, artifact,
  SBOM, signing, and attestation actions.
- Fixtures: floating Node and PostgreSQL image tags were replaced with eligible
  exact tags (`node:24.19.0-alpine` and `postgres:18.6-alpine`), and fixture
  package managers were updated to pnpm `11.22.0` and npm `12.0.2`.

## Security audit

Before the upgrade, `pnpm audit` reported eight high and five moderate
advisories, primarily in transitive parsing, globbing, identifier, HTTP, and CSS
packages. The Go 1.26.5 scan found five reachable standard-library
vulnerabilities. RustSec found no Rust vulnerabilities, but reported inherited
GTK3 maintenance warnings and a medium-severity soundness advisory in the GTK3
`glib` line.

After the upgrade:

- `pnpm audit --audit-level low` reports zero vulnerabilities.
- `govulncheck ./...` reports zero reachable vulnerabilities and zero imported
  package vulnerabilities.
- `cargo audit` reports zero vulnerabilities. Its remaining GTK3/glib warnings
  are described below.

## Deliberate deferrals

- TypeScript `7.0.2` is not adopted in this slice. The newest eligible
  typescript-eslint release declares TypeScript `<6.1.0`, while Switchyard's
  Astro/Vue/OpenAPI toolchain is currently validated on TypeScript `6.0.3`.
  This is an ecosystem migration and should be handled separately once the
  compiler-dependent tools publish compatible releases.
- `type-fest` remains at `2.19.0` because `@testing-library/vue` `8.1.0` imports
  `RemoveIndexSignature` from that major without declaring it as a production
  dependency. Eligible type-fest 4/5 releases are not drop-in replacements.
  Upgrade or remove this compatibility pin together with the testing-library
  migration.
- Tauri's eligible Linux stack still resolves to the unmaintained GTK3 crates
  and `glib` `0.18.5`, which RustSec flags for a medium-severity soundness issue.
  No eligible direct Tauri update removes that graph. Replacing it requires a
  separate Linux desktop backend migration rather than a lockfile-only change.
- anchore/sbom-action `0.24.2`, CodeQL `4.37.9`, and
  softprops/action-gh-release `3.0.3` were published after the cutoff. They
  remain on eligible `0.24.0`, `4.37.8`, and `3.0.2` respectively.

## Verification

- [x] Frozen pnpm install succeeds under Node `26.7.0` and pnpm `11.22.0`.
- [x] Repository checks, Go/Vue formatting and linting, architecture checks,
  Vue type checking, Go unit tests, and Vue unit/coverage tests pass.
- [x] JavaScript, Go, and Rust vulnerability scans complete with no actionable
  vulnerability in the upgraded application dependency graph.
- [ ] Full `make quality` gate.
- [ ] Full public-site quality, browser, and visual gates.
- [ ] Clean generated-code reproducibility check from the committed tree.

## Compatibility note

GolangCI-Lint `2.13.0` adds a cookie-security diagnostic. Switchyard's browser
API is intentionally bound to loopback HTTP by ADR-0013, so setting the cookie's
`Secure` attribute would prevent it from being returned on IP-literal origins.
The existing `HttpOnly` and strict `SameSite` protections remain tested; the
new diagnostic is suppressed only at that cookie construction with this reason.
