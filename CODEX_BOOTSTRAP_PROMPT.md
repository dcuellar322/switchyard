# Codex bootstrap prompt for Switchyard

You are implementing **Switchyard**, a local development command center.

Read these files completely before making changes:

1. `SWITCHYARD_IMPLEMENTATION_PLAN.md`
2. `AGENTS.md`
3. The interactive design reference at `design/switchyard-ui-mockup.html`

Treat the implementation plan as the normative product and architecture specification. Begin with **Phase 0 only**. Do not implement later-phase features prematurely.

Your first task is to:

1. Propose the exact Phase 0 repository bootstrap as a sequence of reviewable PR-sized slices.
2. Identify the bounded contexts and package dependency rules you will establish.
3. List the initial ADR files and their decisions.
4. Define the `make` targets and CI quality gates.
5. Create the repository skeleton, root `AGENTS.md`, initial Go/Vue toolchains, architecture tests, and a Vue application shell that visually follows the supplied mockup.
6. Keep `cmd/switchyard/main.go` and composition code small.
7. Do not add Docker lifecycle, AI, MCP, or agent-session behavior during Phase 0 except minimal compile-time interface placeholders explicitly required by the plan.
8. Run all Phase 0 quality checks and report acceptance-criteria evidence.

Engineering constraints:

- Modular monolith with domain/application/adapter boundaries.
- No god files, services, stores, or components.
- No `utils`, `helpers`, `common`, or service locator.
- DRY, KISS, YAGNI.
- Manual dependency injection.
- Domain types do not import transport or vendor SDK types.
- Generated code is isolated and reproducible.
- Security, typed errors, cancellation, auditability, and testability are designed in from the start.

Before writing code, present your Phase 0 plan and identify any conflict you find in the specification. Otherwise proceed through the phase in small coherent slices and keep the repository buildable after each slice.
