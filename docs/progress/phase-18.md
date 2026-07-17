---
title: "Phase 18: Cross-platform hardening and open-source v1.0"
description: Implementation evidence for Switchyard product phase 18.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-17
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
adapter boundaries in ADR-0002, local IPC in ADR-0003, durable migration rules
in ADR-0004, runtime ownership in ADR-0006, terminal isolation in ADR-0009,
credential indirection in ADR-0010, and signed desktop distribution in ADR-0013.
The stable identifiers are compatibility promises over those accepted
boundaries rather than a new architectural dependency.

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
