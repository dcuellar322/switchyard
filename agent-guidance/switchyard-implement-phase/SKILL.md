---
name: switchyard-implement-phase
description: Implement one documented Switchyard roadmap phase while enforcing domain boundaries, quality gates, and phase exit criteria.
---

# Implement a Switchyard phase

1. Read `AGENTS.md`.
2. Read the requested phase in `SWITCHYARD_IMPLEMENTATION_PLAN.md` and its architectural dependencies.
3. Inspect `docs/progress/` and current code before proposing changes.
4. Write a concise execution plan mapped to every exit criterion.
5. Implement only the requested phase or approved vertical slice.
6. Preserve the modular-monolith dependency direction.
7. Add tests during implementation.
8. Run architecture checks, lint, typecheck, tests, schema/client generation checks, and relevant Playwright verification.
9. Create/update `docs/progress/phase-XX.md` with evidence and unverified items.
10. Run a final review for god files, duplicated knowledge, unsafe command execution, weak cancellation, missing permissions, and visual divergence.

For UI phases, open `design/switchyard-interactive-mockup.html` and verify the production result using Playwright.
