# Phase 4: Complete CLI contract

## Implemented

- Added project list/get/add/trust/remove commands and retained top-level
  `list`/`add` shortcuts.
- Added durable operation list/get/cancel commands with project filtering and a
  bounded result limit.
- Added selection by opaque ID, exact unique slug, or canonical repository path
  with stable not-found and ambiguity errors.
- Added versioned JSON envelopes, JSON Lines for list output, prompt-free
  `--non-interactive`, and `--no-color` compatibility.
- Added semantic exit codes and machine-readable stderr errors.
- Added Bash, Zsh, Fish, and PowerShell completion generation.
- Added manifest explain/diff/validate, primary-endpoint `open`, structured
  `doctor`, and `schema cli` contracts.
- Added safe on-demand daemon startup with private logging, detached process
  configuration, readiness polling, and reuse of an already healthy daemon.
- Repository arguments are resolved against the invoking CLI process before
  daemon startup, so relative paths cannot drift to the daemon's working
  directory. Duplicate adds and repeat trust decisions are reported
  idempotently, and empty unresolved fields render as `none`.
- Added generated project/remove/trust and operation-list endpoints without
  importing persistence or runtime adapters into Cobra commands.

## Exit criteria

- [x] Every query command supports the `switchyard.cli/v1` JSON envelope.
- [x] List output supports one JSON object per item with `--jsonl`.
- [x] No command prompts in automation mode; trust and removal require explicit
  `--yes` instead.
- [x] Invalid and ambiguous project selectors produce stable error codes and
  nonzero semantic exit statuses.
- [x] A packaged `doctor --json` started an absent daemon and returned the same
  schema as subsequent commands against the running daemon.

## Verification

```text
Golden human and JSON output tests: passed
ID, slug, path, missing, and ambiguous selector tests: passed
Client-relative add path, duplicate-add output, and repeat-trust tests: passed
On-demand packaged daemon startup over Unix IPC: passed
Packaged add/list/get/trust/manifest/open/operation/remove workflow: passed
Machine not-found result: PROJECT_NOT_FOUND on stderr, exit 3
```

## Scope guard

Cobra handlers contain transport selection, input resolution, and rendering
only. They call generated clients and do not access SQLite, Docker, Compose,
native-process, or repository scanner adapters directly.
