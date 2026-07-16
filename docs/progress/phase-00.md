# Phase 0: Product charter and architectural guardrails

## Implemented

- Imported the implementation blueprint, repository policy, bootstrap prompts,
  reusable phase guidance, and canonical interactive design reference.
- Established Apache-2.0 licensing, contributor guidance, security reporting,
  community conduct, architecture overview, product glossary, and engineering
  conventions.
- Accepted and indexed all 15 initial architecture decisions from Section 26.
- Added structured bug/feature issue forms and an architecture/test-aware pull
  request checklist.

## Files and modules added

- Root governance and policy documents.
- `docs/architecture`, `docs/conventions`, `docs/adr`, and `docs/progress`.
- `.github/ISSUE_TEMPLATE` and `.github/pull_request_template.md`.
- `design` and `agent-guidance` reference artifacts.

## Architecture decisions

All ADR-0001 through ADR-0015 subjects are accepted. Phase 0's shorthand ADR
numbering conflicts with Section 26; `docs/adr/README.md` records the resolution
to use Section 26's complete numbering.

## Tests added

No production code exists in Phase 0. Documentation structure, local links, and
required policy subjects are verified before the phase commit.

## Commands run and results

```text
git diff --check: passed
ADR inventory check: passed; exactly 15 numbered decision records
required-file and convention-section checks: passed
rg/shasum package inventory: passed; all three HTML design aliases are byte-identical
git status --short: repository was empty and clean before package import
```

## Acceptance criteria status

- [x] A new contributor can explain process topology and dependency direction
  from README and `docs/architecture/README.md`.
- [x] All foundational ADRs are accepted and linked from README.
- [x] No production feature code is present; policy and PR checks require the
  Phase 1 quality pipeline before feature depth.

## Known limitations

- The executable, quality pipeline, and development toolchains begin in Phase 1.
- The issue-template security URL contains an ownership placeholder that must be
  replaced before the public repository is published.

## Deferred work

- Phase 1 walking skeleton, CI jobs, architecture checker, migrations, API
  generation, and visual-test harness.

## Manual verification

- Read `README.md`, `AGENTS.md`, the architecture overview, conventions,
  glossary, and ADR index as a new contributor.
- Confirm the interactive design reference opens and exposes dashboard, project,
  ports, and command-palette states before UI implementation.
