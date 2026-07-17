# Phase 3: Project catalog, manifests, and deterministic discovery

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
- Added the typed `v1alpha1` project manifest, generated draft 2020-12 JSON
  Schema, strict YAML parsing, domain validation, canonical containment,
  executable warnings, port validation, and loopback health-check policy.
- Added five-layer overlay resolution and leaf-level provenance without writing
  portable or local repository files.
- Added transactional proposal persistence, duplicate-root deduplication,
  compare-and-swap approval, trust transitions, audit records, and manifest
  snapshots.
- Added generated onboarding, catalog, manifest explain/diff/validate APIs;
  `switchyard add`; and `switchyard manifest explain|diff|validate` over local
  IPC.
- Added a responsive browser review showing safety posture, validation,
  unresolved fields, services, ports, proposed commands, confidence, and exact
  evidence locations before approval.

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
