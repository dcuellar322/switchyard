# Phase 16: Adapter and plugin SDK

## Implemented

- Added the public `sdk/plugin` Go SDK, bounded JSON-RPC server/client, manifest
  validation, and reusable conformance test kit.
- Added deterministic package discovery, manifest/executable SHA-256 identity,
  explicit trust, reviewed grants, enable/disable, health, redacted logs, and
  fingerprint-change revocation.
- Added per-call external-process supervision with minimal environment,
  protocol/identity negotiation, response validation, timeouts, and Unix
  process-group cleanup.
- Added schema-12 durable plugin registrations and a bounded 1,000-entry log
  history per plugin.
- Added generated REST/TypeScript contracts, a full plugin CLI, and a Vue
  identity/capability/scope review surface with loading, failure, empty,
  changed-identity, disabled, degraded, and healthy states.
- Added a separately built fixture adapter that inspects a real fixture project
  and performs a typed echo action through the same stable contracts.
- Extended the architecture checker so public SDK packages cannot import the
  internal implementation and plugin domain layering is checked like every
  other product domain.
- Published architecture, authoring, compatibility, deprecation, install, and
  local-user trust-boundary documentation.

## Architecture decisions

No ADR changed. This phase implements ADR-0012 and preserves ADR-0010/0011:
plugins remain provider-neutral out-of-process adapters, receive no generic
shell or reverse host-call surface, and route mutations through durable audited
application operations.

## Exit criteria

- [x] The sample external adapter inspects and operates a fixture project through
  the public SDK and the same supervised host runner.
- [x] Plugin exits, panics, timeouts, malformed messages, and identity mismatches
  fail a call without crashing the daemon.
- [x] Undeclared and ungranted scopes are denied before project data or mutation.
- [x] Exact protocol mismatches and changed fingerprints produce actionable,
  fail-closed states.
- [x] Core catalog/runtime/operation domains import no plugin-specific code.

## Verification evidence

Focused SDK, discovery, application, SQLite, process-runner, generated-contract,
CLI, Vue typecheck/lint, and plugin-view tests passed. The external fixture test
builds a separate executable, discovers and trusts its exact fingerprint,
grants scopes, inspects the Node fixture, and executes its typed action. The
complete `make quality` gate passed: generated artifacts, formatting, vet,
GolangCI, architecture checks, all Go and race tests, schema-12 migration,
`govulncheck` with no findings, 13 Vue test files with 22 tests, four browser
workflows, ten visual baselines, production web and Go builds, Rust formatting,
Clippy, six native tests, and debug `.app` plus DMG packaging.
