---
title: "Phase 10: MCP server and agent guidance"
description: Implementation evidence for Switchyard product phase 10.
category: contributor
audience: [contributor, maintainer]
lastVerified: 2026-07-17
---

## Implemented

- Added the official Go MCP SDK and a stdio server backed exclusively by the
  typed local daemon client and existing application use cases.
- Added static `observe`, `develop`, `maintain`, and `admin` profiles with
  optional project allowlists. Tool discovery omits unavailable capabilities;
  observe cannot mutate and destructive tools are admin-only. Scoped proposal
  trust and port-conflict evidence are resolved and filtered by project owner.
- Added bounded structured project, runtime, service, redacted-log, health,
  health-wait, Git, port, action, operation, and effective-manifest reads.
- Added durable start, stop, restart, pause, resume, rebuild, teardown,
  trusted-action, cancellation, proposal-create, and proposal-accept tools.
- Added service-scoped lifecycle planning to the generated REST contract,
  Compose driver, native-process driver, local client, and operation payloads.
- Added 30-second bounded operation and health waits with MCP progress
  notifications and exact terminal-state reporting.
- Added tool read-only, destructive, idempotency, open-world, risk, and minimum
  profile metadata. Trusted action risk is rechecked before submission.
- Added static and templated MCP resources plus onboarding, diagnosis,
  start-and-verify, and port-conflict prompts.
- Added validated agent identity headers on privileged local IPC. Runtime,
  action, cancellation, discovery, trust, and removal paths persist the
  non-secret provider/agent origin in existing audit records.
- Added idempotent Codex and Claude Code project/user installers with atomic
  writes, collision detection, shared Agent Skills, repository guidance, and
  Claude's `@AGENTS.md` import.
- Added SDK transport conformance tests and a pinned MCP Inspector smoke test
  in release-build CI. The Inspector starts a real isolated daemon and separate
  stdio process.
- Added a public MCP schema/tool reference and an architecture note.
- Added a durable default MCP profile. `switchyard mcp serve` loads it from the
  daemon when `--profile` is omitted, while an explicit flag remains the
  authoritative per-session override.

## Files and modules added

- `internal/agents/application`
- `internal/agents/guidance`
- `internal/agents/providers/codex`
- `internal/agents/providers/claude`
- `internal/transport/mcpserver`
- `internal/transport/cli/mcp_command.go`
- `internal/transport/cli/agent_command.go`
- `scripts/test-mcp-inspector.sh`
- `docs/mcp.md`
- `docs/architecture/agent-integration.md`

## Architecture decisions

No new ADR was required. This phase implements ADR-0010's provider-neutral MCP
facade, ADR-0011's profile and no-shell policy, and ADR-0013's privileged local
transport. MCP remains a thin disposable process over a durable daemon. Its
backend interface mirrors explicit application use cases and cannot call
drivers, SQL, Docker, or process execution.

Provider-specific behavior is limited to configuration installation. Both
providers consume the same MCP surface and the same embedded operating skill.
Permission checks are duplicated at the tool-discovery boundary and individual
handler boundary so a stale client tool list cannot elevate authority.

## Tests added

- Profile ranking, unknown capability denial, identity validation, and project
  scoping.
- Codex managed-block preservation, collision refusal, skill/guidance output,
  and repeated-install idempotency.
- Claude JSON merge preservation, repeated-install idempotency, skill output,
  and `CLAUDE.md` import behavior.
- MCP initialize/discovery over official in-memory transports for every
  profile, plus scoped resources, prompts, annotations, bounded logs, service
  restart input, operation wait, and destructive action denial.
- IPC actor-header validation and agent-bound operation audit inputs.
- SQLite persistence of agent identity for catalog proposal audit events.
- Compose and native-process service selection and dependency behavior.
- Real MCP Inspector tool/resource/prompt discovery with observe-profile
  destructive omission.

## Verification evidence

```text
go test ./internal/agents/... ./internal/transport/mcpserver \
  ./internal/transport/httpapi ./internal/transport/cli
PASS

go vet ./internal/agents/... ./internal/transport/mcpserver \
  ./internal/transport/cli ./internal/transport/httpapi ./internal/runtime/...
PASS

go run ./tools/archcheck
PASS

make build
PASS: production Vue bundle and switchyard binary

./scripts/test-mcp-inspector.sh
MCP Inspector smoke passed (tools, resources, prompts; observe profile)

make quality
PASS: generated-code drift, vet, zero lint findings, architecture checks,
TypeScript, Go unit and race suites, migration, vulnerability, 4 live browser
E2E workflows, 6 visual baselines, production web, and production binary build
```

## Acceptance criteria status

- [x] Codex and Claude Code receive the same typed project list, start,
  operation wait, health wait, bounded error query, and targeted service
  restart workflow through their generated project configuration.
- [x] Observe servers do not register any mutation tool, and handler-level
  authorization rejects capabilities or projects outside the server scope.
- [x] Destructive tools are absent from the default observe profile and require
  an explicitly configured admin profile plus relevant confirmations.
- [x] Responses carry a schema version, collection bounds, structured fields,
  and daemon-redacted log content.
- [x] MCP runs as a separate stdio adapter over local IPC; Inspector starts and
  stops MCP processes without affecting the ordinary daemon used across calls.

## Known limitations and deferred work

- Remote HTTP MCP transport is intentionally unavailable. Switchyard's current
  trust model is local stdio plus privileged IPC.
- MCP list pagination uses bounded server pages; the present project catalog
  API is itself a bounded local collection and does not yet expose durable
  cursor pagination.
- AI-generated onboarding candidates belong to Phase 11. Phase 10 proposal
  creation remains deterministic and acceptance remains an explicit admin
  trust decision.
- The Inspector parent package currently requires npm's flattened dependency
  layout for its CLI bundle, so the pinned smoke uses the project's documented
  `npx` launcher rather than `pnpm dlx`.
