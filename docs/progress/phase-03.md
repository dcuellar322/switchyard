---
title: "Phase 3: Project catalog, manifests, and deterministic discovery"
description: Implementation evidence for Switchyard product phase 3.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-20
---

## Implemented

- Added project catalog entities, pending/trusted/rejected trust states,
  repository locations, tags, manifest revisions, proposal lifecycle, and
  immutable accepted snapshots.
- Added canonical-root selection, symlink containment, a one MiB read limit,
  forbidden secret files, and fixed-file discovery without repository code
  execution.
- Added revisioned approved project roots as a control-plane policy. New scans
  outside those canonical directories fail closed unless that single browser
  or CLI request carries an explicit one-shot override; the override is not
  available to MCP proposal creation and never changes repository trust.
- Added independent Git, Compose, Python/uv, Node/npm, Make, Just, README, and
  existing-runtime scanners with exact file and line evidence.
- Added deterministic aggregation of Compose services and ports, lifecycle
  actions, uv/npm/Make/Just commands, tags, confidence by JSON Pointer, and
  unresolved fields.
- Compose discovery models the default profile only, excluding explicitly
  profiled optional services while recording their profile names as trusted
  start-time options, and prefers conventional frontend service names
  when selecting the inferred primary browser endpoint.
- Added the typed `v1alpha1` project manifest, generated draft 2020-12 JSON
  Schema, strict YAML parsing, domain validation, canonical containment,
  executable warnings, port validation, and loopback health-check policy.
- Added five-layer overlay resolution and leaf-level provenance without writing
  portable or local repository files.
- Added transactional proposal persistence, duplicate-root deduplication,
  compare-and-swap approval, trust transitions, audit records, and manifest
  snapshots.
- Pending duplicate-root scans now rerun deterministic discovery and atomically
  supersede the prior proposal, so adding a portable manifest during review no
  longer requires removing and recreating the catalog project.
- Rescanning a trusted project creates a new reviewable proposal while the
  accepted snapshot remains active; accepting it appends a manifest revision
  and refreshes catalog metadata without deleting project history.
- Trust failures distinguish schema validation errors from unresolved fields
  and report their JSON pointers instead of rendering an empty error list.
- Added generated onboarding, catalog, manifest explain/diff/validate APIs;
  `switchyard add`; and `switchyard manifest explain|diff|validate` over local
  IPC.
- Added a responsive browser review showing safety posture, validation,
  unresolved fields, services, ports, proposed commands, confidence, and exact
  evidence locations before approval.
- Added and enforced the complete deterministic fixture inventory from the
  implementation plan, including healthy/degraded/conflicting Compose,
  mixed-runtime, monorepo, external-process, worktree, adversarial README, and
  secret-redaction scenarios.

## Architecture decisions

Phase 3 applies ADR-0001 domain boundaries, ADR-0004 generated transport
contracts, ADR-0005 transactional persistence, and ADR-0008 manifest precedence
and provenance. Discovery remains deterministic application behavior and does
not execute repository code.

## Security behavior

- `.env` and machine-specific environment files are never opened by discovery.
- Compose environment values and Node script bodies are not returned.
- Escaping symlinks and manifest paths are rejected.
- Credential-like README titles are redacted.
- Unresolved or invalid proposals cannot be trusted.
- Browser mutations retain session, CSRF, and idempotency-key enforcement.

## Exit criteria

- [x] The packaged `switchyard add test/fixtures/mixed-project` command created
  a reviewable proposal over a private Unix socket.
- [x] The mixed fixture inferred two Compose services, host/target ports, uv
  tests, npm scripts, Make targets, and Just targets.
- [x] Unknown manifest YAML fields fail strict decoding.
- [x] The local overlay wins and the portable file remains byte-identical.
- [x] Every evidence item has a source path and valid one-based line range.
- [x] `.env` secret canaries are absent from proposals and browser output.
- [x] An outside-root scan is rejected by default, an explicit override permits
  only that deterministic scan, and symlink/filesystem-root settings are
  rejected during settings validation.

## Verification

```text
Go manifest/discovery/catalog/SQLite tests: passed
Pending proposal refresh and actionable trust-error regressions: passed
Trusted-project rescan, optional Compose profile, and frontend endpoint regressions: passed
Go full package suite and architecture check: passed
Vue typecheck, ESLint, and Vitest: passed
Playwright: system and evidence-backed onboarding flows passed
Real binary + daemon + Unix IPC add fixture: passed; valid proposal, schema 3
Generated JSON Schema/OpenAPI/sqlc/TypeScript outputs: reproduced
Root-policy application, HTTP, CLI, and persistence tests: passed
```

## Scope guard

AI is not used in scanning or proposal aggregation. Ambiguous facts remain
explicitly unresolved. Runtime commands are displayed but never executed by
onboarding. Compose and native-process lifecycle execution begin in later
phases.

## 2026-07-20 local project compatibility follow-up

### Implemented

- Bumped deterministic proposal identification to `deterministic/v2` after
  expanding inference behavior.
- Added Compose host-port parsing for numeric defaults in `${NAME:-PORT}` and
  `${NAME-PORT}` expressions, including host-IP mappings and long syntax.
  Ports without a deterministic numeric host value remain absent from the
  candidate and now produce exact-line warning evidence.
- Added reviewable fallback discovery for a single conventional local/dev
  Compose filename when no standard Compose filename exists.
- Added package-manager detection from safe lockfile existence checks without
  reading potentially large lockfiles. Node `dev`/`start` scripts now produce
  a one-process runtime proposal through the package manager's argument-array
  command.
- Added process inference from PEP 621 scripts and from a deliberately narrow,
  shell-free uvicorn grammar in fenced README examples. Unknown flags, path
  options, shell operators, and malformed ports fail closed.
- Split Compose scanning into its own adapter file so scanner responsibilities
  remain below repository review thresholds.

### Tests and verification

- Added regressions for Compose variable-default ports, exact unresolved-port
  evidence, local Compose filenames, pnpm lockfile detection, inferred Node
  processes, documented uvicorn processes, oversized lockfiles, escaping
  symlinks, shell operators, and unsafe README path options.
- A temporary compiled daemon scanned the runnable project definitions under
  `/Users/dac/dev` without executing repository code or reading `.env` files.
  All ten runnable definitions produced valid proposals with no unresolved
  fields: standard Compose, local Compose, Node process, documented uvicorn,
  and explicit Switchyard-manifest shapes were all represented.
- Non-runnable directories (documents, certificates, raw source snippets,
  generated internals, and empty directories) remained unresolved rather than
  receiving fabricated lifecycle commands.

```text
make lint
PASS: gofmt, Prettier, go vet, golangci-lint, architecture checks, ESLint

make test
PASS: all Go packages; 23 Vitest files / 49 tests

make test-race
PASS: all Go packages on the final run
NOTE: the first run saw a transient EOF in the unrelated plugin process
fixture; its isolated race test and the complete rerun both passed.

make generate-check repository-check
PASS: manifest schema, OpenAPI Go/TypeScript, sqlc, repository policy, diff

make build
PASS: production Vue bundle and local bin/switchyard 1.0.0

temporary compiled daemon compatibility matrix
PASS: 10/10 runnable definitions valid, 0 unresolved fields
```

### Known limitations

- Discovery does not recursively guess project roots. A nested stack is added
  at its actual root (for example, the directory containing its Compose file).
- Framework-specific browser ports are not guessed for Node processes. Their
  managed process lifecycle, logs, and metrics work, while endpoints can be
  added during review or through a local overlay.
- The audited declarations include one duplicate host port across otherwise
  independent projects. The existing Phase 8 port registry reports that
  conflict; concurrent operation requires a reviewed local override.
