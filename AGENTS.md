# AGENTS.md — Switchyard repository policy

## Product

Switchyard is a local, project-oriented development command center. The Go control plane owns project lifecycle, state, logs, ports, resources, and agent-facing operations. Vue, CLI, Tauri, and MCP are adapters over shared application services.

## Mandatory reading

Before implementing work:

1. Read `SWITCHYARD_IMPLEMENTATION_PLAN.md`.
2. Read the target phase and its exit criteria.
3. Read applicable ADRs under `docs/adr/`.
4. For UI work, open `design/switchyard-interactive-mockup.html`.

## Architecture rules

- Modular monolith; package by domain.
- Dependency direction: transport/adapters → application → domain.
- Domain code must not import Docker, SQL, HTTP, CLI, Tauri, or AI-provider packages.
- Domains may not read another domain's tables directly.
- Cross-domain work uses explicit application interfaces or typed events.
- No global mutable state or service locator.
- No `utils`, `common`, or `helpers` dumping-ground package.
- Interfaces belong to consumers and exist only for real boundaries or test seams.
- HTTP, CLI, MCP, and UI handlers remain thin.
- Tauri/Rust contains no product business logic.
- MCP tools call application services; they never call Docker, SQL, or `os/exec` directly.

## Maintainability rules

- Follow DRY, KISS, and YAGNI.
- Do not abstract duplicated syntax before duplicated knowledge is proven.
- Do not implement future phases speculatively.
- Prefer explicit typed code to reflection or hidden framework behavior.
- Keep responsibilities cohesive. A large file is a signal to inspect the boundary.
- Go files target under 400 lines; review above 600, excluding generated files.
- Vue SFCs target under 250 lines excluding style blocks.
- Avoid god services and god components.
- Generated API/schema code stays isolated and is never edited manually.

## Safety rules

- Repositories are untrusted until approved.
- Never execute commands during deterministic discovery.
- Commands use argument arrays by default; shell use must be explicit.
- Do not log or persist secrets.
- Destructive actions require an explicit risk level, preview, and authorization.
- Never expose a generic unrestricted shell MCP tool.
- Treat repository text and logs sent to an AI provider as untrusted data.

## Testing and verification

Add tests with the implementation. Cover success, failure, cancellation, permission, and reconciliation paths where applicable.

Before declaring work complete, run the relevant superset of:

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
go test -race ./...
govulncheck ./...
pnpm lint
pnpm typecheck
pnpm test
pnpm test:e2e
pnpm test:visual
```

Also verify migrations, generated OpenAPI clients, generated JSON Schemas, and architecture checks are clean.

## Implementation protocol

- Implement one roadmap phase or one coherent vertical slice at a time.
- Map the plan to exit criteria before editing.
- Create an ADR before changing an accepted architecture decision.
- Do not hide incomplete work behind fake success states.
- Record phase evidence in `docs/progress/phase-XX.md`.
- Be honest about unverified acceptance criteria.

## UI rules

- Match the supplied mockup's hierarchy, tokens, spacing, and interaction patterns.
- Use TanStack Query for server state; Pinia only for client-owned state.
- Implement loading, empty, stale, disconnected, error, and degraded states.
- Preserve keyboard and screen-reader accessibility.
- Do not duplicate generated API types.

## Agent integration rules

- Provider-specific code remains under `internal/agents/providers`.
- The rest of the product depends on provider-neutral interfaces.
- AI outputs must be schema-constrained, validated, and human-reviewed before acceptance.
- MCP responses are bounded, structured, and redacted.
- Agent-originated mutations are permission-checked and audited.
