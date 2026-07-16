# Phase 17: Intelligent diagnosis and safe automation

## Implemented

- Added bounded structured diagnostic bundles spanning project identity,
  runtime, health, redacted logs, Git, ports, resources, configuration
  provenance, recent operations, accepted actions, and preview-only cleanup.
- Added deterministic rules for runtime disconnection/degradation, required
  health failures, repeated crashes, port conflicts and bind errors, common log
  signatures, resource pressure, incomplete Git operations, and stale projects.
- Added provider-neutral diagnosis generation with strict JSON Schema, byte
  budgets, evidence/action citation validation, isolated provider prompts, and
  deterministic results preserved across every provider failure mode.
- Added durable diagnoses, local-only accuracy feedback, deduplicated alerts,
  disabled-by-default automation recipes, cooldown/daily-run limits, and
  bounded per-project receipt retention. The bounded desktop snapshot bridges
  new or repeated diagnostic alerts into native OS notifications.
- Added permission-checked one-click action submission and automation dispatch
  through the ordinary durable operation kernel; diagnostics never execute a
  command or expose deletion, source editing, networked, destructive, or
  interactive automation.
- Added generated REST and TypeScript contracts, CLI diagnosis/automation
  commands, a responsive browser review surface, component coverage, persistence
  and transport tests, and a reviewed visual baseline.
- Published the evidence, prompt-injection, permission, retention, notification,
  feedback, and automation architecture and CLI contracts.

## Architecture decisions

No ADR changed. This phase implements ADR-0010, ADR-0011, and ADR-0014:
providers remain behind neutral application interfaces, no generic shell or
implicit privilege is introduced, and untrusted evidence is redacted and
bounded before persistence or dispatch.

## Exit criteria

- [x] Common known failures are diagnosed deterministically.
- [x] AI suggestions cite local evidence and cannot bypass permissions.
- [x] No diagnostic automatically deletes data or edits source by default.
- [x] Users can inspect and disable every automation.
- [x] False-positive feedback can be recorded locally without sending telemetry.

## Verification evidence

Focused diagnostic rule/provider/automation, SQLite retention and state,
HTTP authorization, provider adapter, generated contract, CLI, Vue component,
typecheck, lint, and browser visual tests passed. The visual baseline was
inspected at full resolution and shows deterministic provenance, confidence,
evidence identifiers, redacted/untrusted state, approved actions, cleanup dry
run, local alerts, acknowledgment, and explicit recipe limits. The complete
`make quality` gate passed: generated artifacts, formatting, vet, GolangCI,
architecture checks, all Go and race tests, schema-13 migration,
`govulncheck` with no findings, 14 Vue test files with 25 tests, four browser
workflows, 11 visual baselines, production web and Go builds, Rust formatting,
Clippy, six native tests, and debug `.app` plus DMG packaging.
