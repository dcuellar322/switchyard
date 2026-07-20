---
title: "Phase 18: Cross-platform hardening and open-source v1.0"
description: Implementation evidence for Switchyard product phase 18.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-20
---

## Implemented

- Replaced platform assumptions with native Linux and Windows adapters: portable
  listener inspection, Linux terminal/editor launch, owner-only Windows named
  pipes, Job Object process-tree ownership, ConPTY terminals, Windows Credential
  Manager reads, and reviewed URL launch validation.
- Promoted the product, project manifest, plugin protocol, plugin manifest,
  generated schema, desktop shell, and examples to stable `1.0.0`/v1 contracts.
  Legacy alpha manifests normalize in memory and have an explicit preview-first,
  backup-preserving file migration command.
- Added offline database inspection, explicit backup, and preview/write migration
  commands. Automatic upgrades now verify the source and a private consistent
  backup before applying migrations, refuse newer schemas, and preserve projects,
  accepted manifests, audit history, settings, and all other durable records.
- Added native adapter and sidecar CI matrices, a cross-platform sidecar builder,
  GoReleaser archives/checksums/CycloneDX SBOMs, platform desktop bundles,
  Apple/Windows/Linux signing hooks, Tauri updater signatures, Sigstore bundles,
  GitHub artifact attestations, CodeQL, dependency review, and secret scanning.
- Added fail-closed nightly/alpha/beta/stable tag classification, exact-tag
  checkout for manual releases, prerelease marking, and a scheduled unsigned
  cross-platform nightly CLI snapshot with short artifact retention.
- Published the v1 compatibility/deprecation policy, platform and WSL behavior,
  data migration and restore-based downgrade procedures, release matrix,
  contributor setup, adapter guide, manifest reference, privacy policy, support
  policy, troubleshooting guide, threat model, and runnable Compose/process
  examples.

## Architecture decisions

No accepted dependency direction changed. This phase completes the native
adapter and platform-order boundaries in ADR-0002 and ADR-0015, durable
migration rules in ADR-0005, runtime and terminal process ownership in ADR-0007,
the thin signed desktop shell in ADR-0009, local IPC and browser security in
ADR-0013, and optional authenticated federation in ADR-0016. The stable
identifiers are compatibility promises over those accepted boundaries rather
than a new architectural dependency.

## Exit criteria

- [x] Primary workflows have native macOS coverage and required Linux/Windows
  host matrices; the release checklist records the manual installer matrix.
- [x] Upgrade fixtures prove projects, accepted manifests, audit history, and
  preferences survive; every write migration creates and verifies a backup.
- [x] The v1 security review has no unresolved critical or high findings.
- [x] CLI, manifest, HTTP, MCP, plugin, and product compatibility and deprecation
  policies are public in the repository.
- [x] A new contributor can install, generate, test, build, and run the project
  from documented commands, working example manifests, and the enforced
  §19.2 fixture inventory.

## Verification evidence

The complete local quality gate passed on macOS: generated contracts, Go and
Rust formatting, vet, GolangCI, architecture checks, all Go and race tests,
schema migration tests, `govulncheck`, Vue lint/typecheck/unit coverage, browser
workflow and visual suites, production web/Go builds, Clippy, Rust tests, and
debug desktop packaging. Linux and Windows adapter suites compile from the
local checkout and are required as native hosted jobs before merge. The
GoReleaser v2 configuration and every GitHub workflow pass their configuration
linters. A local v1 daemon smoke test now reports API v1/schema 17; database
upgrade coverage preserves the revisioned settings singleton and value-free
audit history alongside the earlier project, manifest, and operation records.
The smoke discovered and approved the repository deterministically, listed the
project, inspected the database, and shut down cleanly.

Release-channel unit tests reject unknown or injection-shaped tags and classify
nightly, alpha, beta, and stable forms deterministically. The manual release
path resolves an existing tag before any build job begins; scheduled nightly
artifacts never receive stable signing or updater authority.

Signed/notarized release artifacts are deliberately created only by a reviewed
`v*` tag in the protected release environment. The workflow refuses partial
publication and leaves a draft release for human inspection; no signing key or
production release was fabricated during local verification.

## 2026-07-17 cross-platform regression evidence

- The forced-stop integration helper now ignores both POSIX `SIGTERM` and
  Windows `os.Interrupt`, matching the CTRL_BREAK path used by the Windows
  supervisor and preserving the expected `stopped_forced` result.
- Native macOS adapter tests pass, and the Linux and Windows adapter packages
  cross-compile from this checkout. The corrected Windows behavior remains
  required on the hosted Windows runner.

## 2026-07-20 V1 hardening evidence

The V1 security pass closed the findings identified in local browser, process,
filesystem, transport, certificate, persistence, and static-analysis review:

- Browser sessions now have idle and absolute expiry, browser mutations require
  same-origin requests and constant-time CSRF validation, dynamic API responses
  are non-cacheable, correlation identifiers are bounded and validated, and
  non-loopback browser listeners fail closed.
- Plugin and agent executables are resolved through symlinks, confined to their
  approved roots, checked for regular executable files, fingerprinted again
  immediately before launch, and supervised as whole process trees on Unix and
  Windows. Terminal Compose file paths use the same confinement rules.
- Private keys are accepted only from regular owner-private files. Telemetry
  does not follow redirects, fleet TLS validation handles the complete peer
  chain, support bundles publish without overwriting existing files, and agent
  installer outputs cannot escape through symlinks.
- Unsigned-to-signed conversions and persisted counters are range checked or
  saturated. Log deletion uses a rooted filesystem handle, workspace SQL uses
  explicit statements, and GolangCI/gosec reports zero findings under the
  repository's documented boundary-specific exclusions.

The production-readiness and maintainability pass also addressed behavior found
under real UI load:

- Runtime observations use a one-second, per-project singleflight snapshot with
  clone isolation and lifecycle invalidation. Port registry observations use a
  ten-second coalesced snapshot while safety-sensitive port suggestions always
  refresh. The global sidebar no longer triggers expensive host-wide listener
  scans, and project port data loads only on the Ports tab.
- Workspace project states are translated exhaustively into the operation
  kernel's durable step states. This fixes a constraint failure discovered by
  the browser start/stop flow and is covered by regression tests.
- Nine oversized Vue views and panels were split into domain-specific
  composables/components, dead UI code and duplicate mockups were removed, and
  architecture checks now enforce the 250-line non-style Vue target. Prettier is
  pinned and enforced alongside ESLint and the generated API type boundary.
- Playwright uses one worker, strict ports, and isolated non-reused servers, so
  a developer's unrelated local application cannot silently satisfy a test
  server probe.

Fresh verification from this checkout passed:

- `make lint`, `make test`, and `make test-race`; the frontend suite contains 49
  tests across 23 files and meets all configured coverage thresholds.
- `make test-e2e` with five browser workflows, including terminal persistence,
  a native process lifecycle, workspace lifecycle, and a real Docker Compose
  build/start/observe/stop/teardown flow.
- `make test-visual` with 14 Chromium visual and responsive-state checks.
- `make generate-check`, `make migrate-check`, `make platform-check`,
  `make test-plugin-sdk`, production web and Go builds, and the MCP Inspector
  tools/resources/prompts smoke test.
- `govulncheck` found no reachable or imported Go vulnerabilities; the one
  advisory in a required module is not called. `pnpm audit --audit-level high`
  found no known JavaScript vulnerabilities. A local RustSec scan of all 533
  locked desktop crates found no vulnerability advisories. It reported 17
  informational maintenance/unsoundness warnings in Tauri transitive crates;
  the affected `glib::VariantStrIter` API is not used by Switchyard and the
  patched `glib` series is not compatible with the latest Tauri Linux GTK3
  graph. The security workflow scans the nested desktop lockfile so upstream
  fixes cannot pass unnoticed.
- Rust formatting, strict Clippy, all 11 Rust tests, and native macOS debug app
  and DMG packaging passed. The documentation site passed generation, type and
  content checks, lint, 19 tests, production build, and validation of 141 pages.

This local evidence does not substitute for the required hosted native Linux
and Windows jobs or release-environment signing, notarization, updater signing,
attestation, and artifact publication. The local desktop bundles are unsigned
debug artifacts; reviewed release credentials remain confined to the protected
release environment.

## 2026-07-20 pull-request CI follow-up

The first hosted V1 pull-request run exposed Windows-only assumptions that
cross-compilation cannot execute: plugin protection checked Unix mode bits,
support fixtures inferred ACLs from portable file modes, golden files inherited
CRLF checkout conversion, JSON fixtures embedded unescaped Windows paths, and
SQLite file URLs did not normalize drive-letter paths. The plugin process test
also stripped the `.exe` suffix from its copied helper. These boundaries now
use platform-specific executable policy, platform-neutral fixtures, preserved
executable suffixes, forced-LF goldens, and one Windows-safe SQLite DSN
constructor. The Go and frontend formatting checks are also split between jobs
with their required toolchains.

Focused package tests, both formatting gates, `make platform-check`, and the
full `make lint` gate pass locally. Hosted Windows execution and the remaining
pull-request workflows remain the merge gate.
