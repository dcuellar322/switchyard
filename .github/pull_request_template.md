## Outcome

<!-- Describe the user-visible and architectural result. -->

## Scope

- Roadmap phase or approved slice:
- Related issue/ADR:
- Explicitly deferred:

## Architecture checklist

- [ ] Adapter → application → domain dependency direction is preserved.
- [ ] No domain reads another domain's tables.
- [ ] Handlers, CLI commands, MCP tools, and Tauri code remain thin.
- [ ] No god file/component, service locator, global mutable state, or generic helper package was added.
- [ ] Command execution, permissions, cancellation, redaction, and destructive-risk behavior are explicit where applicable.
- [ ] Contract/schema changes are generated and reproducible.
- [ ] An ADR was added if an accepted decision changed.

## Test checklist

- [ ] Success and relevant failure/cancellation/permission/reconciliation paths are covered.
- [ ] Focused tests pass.
- [ ] Full applicable quality gates pass.
- [ ] Migrations apply from empty and supported upgrade fixtures when changed.
- [ ] UI states, accessibility, and Playwright visual evidence are updated when changed.
- [ ] `docs/progress/phase-XX.md` records commands, results, limitations, and manual verification.

## Verification evidence

<!-- Exact commands and concise results. Do not include secrets. -->
