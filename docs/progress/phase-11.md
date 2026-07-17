---
title: "Phase 11: AI-assisted onboarding and manifest generation"
description: Implementation evidence for Switchyard product phase 11.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-17
---

## Implemented

- Added a consumer-owned, provider-neutral proposal interface, capability
  registry, limits, usage, exact evidence receipt, field review, conflict,
  dry-run, and durable run models.
- Added bounded repository excerpts with symlink containment, structural
  environment/secret-reference removal, canonical credential redaction,
  deterministic ordering, total byte limits, truncation, exact preview bytes,
  and a SHA-256 receipt.
- Added Codex CLI generation in an empty temporary root with read-only sandbox,
  ephemeral sessions, ignored user config/rules, stdin evidence, allowlisted
  environment, bounded streams, process-group cancellation, and JSON Schema
  output.
- Added Claude Code print-mode generation with plan permissions, bare and
  no-session modes, no tools, JSON Schema output, and turn/cost ceilings.
  Missing Claude Code is an ordinary unavailable capability.
- Added a generic configured OpenAI-compatible Chat Completions provider with
  Structured Outputs, no tools, bounded bodies, disabled redirects, local HTTP
  restrictions, and process-owned endpoint/model/API-key configuration.
- Added typed merge rules that compute confidence from verified evidence IDs,
  retain high-confidence deterministic facts, record conflicts, reject
  hallucinated files/tools/services/ports/actions and secret requests, and
  preserve deterministic lifecycle and resource policy.
- Added side-effect-free canonical dry-run validation and immutable assisted
  proposal revisions. Provider generation cannot accept a proposal; agent
  identities are forbidden from trusting assisted revisions.
- Added a cancellable `manifest.enhance` durable operation and SQLite run
  receipt containing the exact sent bundle, limits, field provenance,
  conflicts, dry-run, usage, and redacted terminal error.
- Added generated REST/TypeScript contracts for availability, evidence preview,
  generation, and run review.
- Added browser evidence consent, unavailable/degraded provider states,
  cancellation, exact JSON inspection, field confidence/evidence/warnings,
  rejected suggestions, deterministic conflicts, and dry-run status while
  preserving deterministic scan/validate/approve.

## Architecture decisions

No new ADR was required. This phase implements ADR-0008's precedence and
field-provenance policy, ADR-0010's provider-neutral and deterministic-first AI
boundary, ADR-0011's no-generic-shell permission model, ADR-0013's local
transport policy, and ADR-0014's redaction-before-AI requirement.

The provider never receives a repository root or application service. CLI and
HTTP adapters receive only sanitized JSON plus a response schema. Catalog and
manifest behavior remain behind explicit consumer-owned application ports.

## Tests added

- A hermetic ambiguous Node/Python/Make fixture that remains unresolved without
  AI and receives a valid reviewable process proposal through the real scanner,
  catalog, SQLite migration, evidence builder, provider contract, typed merge,
  dry-run, and revision store.
- Exact preview/sent/persisted byte equality and SHA-256 verification, absolute
  root omission, `.env` exclusion, secret redaction, and inert prompt-injection
  evidence.
- Malformed, unknown-field, multi-document/oversized output, invalid JSON
  Pointer, unknown evidence ID, hallucinated port/action, environment/secret
  request, and high-confidence conflict cases.
- Provider cancellation without revision creation, durable cancelled receipts,
  and idempotent succeeded-run recovery.
- Codex read-only/schema/ephemeral/ignored-config arguments, stdin-only bundle,
  temporary root, private schema file, and daemon-environment minimization.
- Claude structured result/usage decoding and unavailable capability behavior.
- OpenAI-compatible structured request, no-tools contract, exact bundle,
  bounded usage, redirect refusal, URL credential refusal, and public plaintext
  endpoint refusal.
- Human-only assisted proposal acceptance and SQLite exact run receipt roundtrip.
- UI evidence-consent, unavailable-provider, redaction, field-rejection,
  conflict, and failed dry-run states.

## Acceptance criteria status

- [x] The complex ambiguous fixture receives a valid reviewable proposal from a
  hermetic provider through the production provider-neutral boundary.
- [x] Deterministic onboarding remains unchanged and is covered by the mixed
  project browser/unit flow with every AI provider optional.
- [x] Discovery executes no repository command; providers receive only an
  immutable bundle and CLI providers run in an empty root with no repository
  path.
- [x] Hallucinated files, tools, ports, services, actions, shell/environment,
  and secret requests are rejected or surfaced as rejected field reviews.
- [x] The UI/API show the exact immutable provider payload, byte/redaction
  counts, evidence lines, truncation, and SHA-256 receipt before consent.

## Verification evidence

`make quality` passed on 2026-07-16. The gate regenerated and diff-checked the
OpenAPI, JSON Schema, SQL, Go, and TypeScript artifacts; ran `gofmt`, `go vet`,
golangci-lint with zero findings, the architecture checker, ESLint, and all
TypeScript type checks; and completed all Go and web unit tests.

The same gate also passed `go test -race ./...`, the empty-database migration
test at schema version 7, `govulncheck ./...` with no vulnerabilities, four
daemon-backed Playwright end-to-end tests (including real process and Compose
lifecycles), six visual-regression tests, the Vite production build, and the
trimmed Go binary build.

## Known limitations and deferred work

- Codex CLI exposes read-only sandboxing rather than a no-tools CLI switch. It
  therefore runs in a fresh empty root with no repository path; Claude Code is
  additionally given an empty tool set.
- Generic OpenAI-compatible endpoints vary in their Structured Outputs support.
  Incompatible endpoints fail the bounded run without modifying the source
  proposal.
- Provider configuration is process-level in Phase 11. A settings UI and
  operating-system credential-store configuration can be added in a later
  coherent settings slice without changing the provider contract.
