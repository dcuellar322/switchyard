---
title: "Phase 19: Post-v1 optional expansion"
description: Implementation evidence for Switchyard product phase 19.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-17
---

## Implemented

- Added optional remote Switchyard agents over user-owned tunnels with TLS 1.3
  mutual authentication, exact certificate pins, protocol negotiation, explicit
  capability grants, typed lifecycle operations, bounded inventory, and durable
  audit records.
- Added multi-machine inventory, offline/error reconciliation, grant revocation,
  remote development-environment visibility, and a responsive read-only
  companion route that omits trust and mutation controls.
- Added Ed25519-signed `switchyard.bundle/v1` configuration bundles with
  reviewed publishers, restrictive policy merging, portable project templates,
  enterprise settings, and curated plugin metadata that never auto-installs or
  grants trust.
- Added age X25519 encrypted configuration sync. Decryption remains in memory,
  previews conflicts before import, applies atomically, and excludes secrets,
  runtime state, logs, credentials, telemetry, and local trust decisions.
- Added opt-in anonymous usage counters with no default endpoint. The complete
  fixed-schema payload and HTTPS destination are shown before consent, policy
  may deny collection, and opt-out clears pending counters permanently.
- Added generated OpenAPI/Go/TypeScript contracts, stable CLI commands, Vue
  management surfaces, visual baselines, privacy and federation guidance, and
  schema migrations 14 through 16.

## Files and modules added

- `internal/fleet/{domain,application,adapters}` and
  `internal/platform/sqlite/fleet.go` own remote identity, authorization,
  reconciliation, transport, and audit persistence.
- `internal/team/{domain,application,adapters}` owns signed bundles, templates,
  policy, registry metadata, verification, and encrypted sync.
- `internal/telemetry/{domain,application,adapters}` owns bounded counters,
  consent, policy enforcement, payload preview, delivery, and opt-out.
- `internal/transport/{cli,httpapi,httpclient}` and `api/openapi.yaml` expose the
  same application services through generated contracts.
- `web/src/domains/{fleet,team,telemetry}` implements the authenticated machine,
  companion, shared-configuration, and consent experiences.
- `migrations/00014_fleet.sql`, `00015_team_configuration.sql`, and
  `00016_telemetry.sql` add durable schema version 16.

## Architecture decisions

ADR-0016 accepts optional direct federation without a hosted control plane.
Local-only mode remains the default, remote listeners are absent unless fully
configured, TLS identity and application authorization are separate, remote
mutations reuse typed application operations, and shared configuration cannot
transport secrets or local trust. Domain packages remain independent of HTTP,
TLS, SQL, Docker, command execution, and encryption adapters.

## Tests added

- Mutual-TLS, certificate-pin, protocol-version, controller-identity, capability,
  confirmation, permission, failure reconciliation, and audit tests cover the
  machine controller and agent paths.
- Signature, publisher trust, tamper rejection, restrictive policy, template,
  curated registry, atomic sync, wrong-recipient, and persistence tests cover
  shared configuration.
- Default-off consent, fixed-counter allowlisting, policy denial, endpoint
  review, delivery, opt-out clearing, and persistence tests cover telemetry.
- Vue unit tests cover authenticated inventory, read-only companion behavior,
  publisher review, encrypted-sync exclusions, and exact telemetry consent.
  Visual tests cover signed configuration and telemetry states.

## Commands run and results

- `make quality` passed end to end: generated-code reproducibility; Go and Rust
  formatting; vet; zero-warning GolangCI and Clippy; architecture checks; Vue
  lint and type checking; all Go, race, Vue, and Rust tests; migrations; Linux
  and Windows compile checks; `govulncheck`; four browser workflows; thirteen
  visual baselines; production web and Go builds; and native macOS app/DMG
  packaging.
- `pnpm --dir web build` passed with route-level code splitting. The initial
  JavaScript chunk is 92.64 kB and no chunk-size warning remains.
- A fresh schema-17 daemon started on loopback from `bin/switchyard`. Stable JSON
  smoke checks reported API v1/status ready, telemetry disabled with no pending
  counters, and empty machine and publisher inventories before clean shutdown.

## Acceptance criteria status

- [x] Every remote feature preserves local-only mode.
- [x] Remote actions have explicit identity, authorization, and audit.
- [x] No cloud dependency is introduced for existing local workflows.

## Known limitations

- Federation deliberately requires user-managed reachability and PKI. Switchyard
  does not ship a relay, account system, certificate authority, or hosted broker.
- The mobile companion is a responsive read-only web surface, not a separately
  distributed native mobile application.
- Curated registries distribute signed metadata only; installation, executable
  trust, scopes, and enablement remain separate local reviews.
- Telemetry has no built-in collector. Opt-in requires an explicitly configured
  HTTPS endpoint so the repository never silently chooses a recipient.

## Deferred work

Hosted relays, centralized accounts, native mobile clients, fleet-wide secret
sync, registry auto-installation, and production telemetry infrastructure remain
outside the accepted local-first architecture. Adding any of them requires a
new demand signal, threat model, and ADR.

## Manual verification steps

1. Run `make quality` with Go, Node/pnpm, Docker, and the pinned Rust toolchain.
2. Start `bin/switchyard daemon --data-dir <temporary-directory>` and verify
   `doctor`, `machine list`, `team publisher list`, and `telemetry status` using
   `--json`.
3. Open the Fleet, Companion, Team, and Settings routes and confirm that remote
   mutations require explicit grants/confirmation, Companion remains read-only,
   publisher trust is reviewable, and telemetry displays its exact payload and
   destination before consent.
