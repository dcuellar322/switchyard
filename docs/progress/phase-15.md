# Phase 15: Desktop application

## Implemented

- Added a pinned Tauri 2/Rust native shell and generated platform icon assets
  from the Switchyard mark.
- Bundled the trimmed Go binary as a target-triple sidecar with exact version,
  commit, and build-time identity.
- Added a typed, versioned `desktop snapshot` CLI boundary for daemon, host,
  project runtime/health, workspace, recent-operation, and port-conflict state.
- Added exact desktop/bundled-sidecar and compatible same-major daemon/API/
  minimum-schema preflight before browser credential issuance or native
  mutations.
- Added a migration rollback guard: a binary refuses a database newer than its
  embedded migrations before Goose attempts an upgrade.
- Added a live tray with daemon state and bounded project/workspace open,
  start, and stop actions; close-to-tray preference; optional launch-at-login;
  single-instance focus; and project/workspace deep links.
- Added transition-based native notifications for failed/partial operations,
  project health, daemon connectivity, Docker connectivity, port conflicts,
  and high host resource use.
- Added a release-only, public-key-verified updater and a protected GitHub
  workflow for signed, notarized, draft macOS releases.
- Added macOS desktop CI, source-build/headless installation guidance, and
  explicit uninstall/data-retention choices.

## Architecture decisions

No ADR changed. The implementation applies ADR-0003's bundled Go control plane,
ADR-0009's thin Tauri adapter, ADR-0013's authenticated loopback boundary, and
ADR-0015's macOS-first platform order. Rust contains native presentation and
fixed command mappings only; all product behavior remains behind Go application
services.

## Safety properties

- The Vue loopback origin has no Tauri capabilities. Native plugins are called
  only by Rust, and no arbitrary shell command is exposed.
- Deep links, UI routes, resource identifiers, sidecar envelopes, response
  size, version, API, and database schema are validated before use.
- Raw sidecar stderr and repository/log payloads are never copied into desktop
  notifications.
- First observation does not replay old failures. Health, daemon, and resource
  notices are edge-triggered rather than emitted every poll.
- Closing or uninstalling the adapter never deletes state, repositories,
  containers, volumes, or logs. Newer databases fail closed under older code.
- Release builds require updater and Apple signing material from a protected CI
  environment; local debug builds have no update authority.

## Tests added

- Database-newer-than-binary rejection before migration.
- Local UI path validation for remote origins, network paths, dot/encoded-dot
  traversal, fragments, backslashes, and injected bootstrap credentials.
- Exact desktop/sidecar plus same-major daemon/API/minimum-schema compatibility
  acceptance, additive-schema acceptance, and fail-closed mismatch cases.
- Deep-link route parsing and rejection of traversal, foreign origins, query
  injection, and malformed identifiers.
- Sidecar resource-identifier containment and versioned command response
  validation.
- Notification initialization, operation/health transitions, warning
  deduplication, and recovery.

## Acceptance criteria status

- [x] Desktop behavior is the same Go application behavior used by browser,
  CLI, and MCP clients.
- [x] Closing the window follows the persisted user preference and never
  fabricates daemon shutdown.
- [x] The tray reflects daemon/project/workspace state and uses fixed typed
  actions.
- [x] Incompatible sidecar/daemon/API/minimum-schema combinations are detected
  before native mutation; newer databases are also rejected by Go before
  migration.
- [x] Installation supports signed/notarized macOS releases and a CLI-only
  headless path.
- [x] Uninstall guidance offers preserve, backup, and reviewed-delete data
  choices without implying repository or Docker-volume deletion.

## Verification evidence

Focused verification on 2026-07-16 passed:

- the Go SQLite, CLI, and HTTP-client packages passed their focused tests;
- six Rust unit tests passed and Clippy with `-D warnings` was clean;
- the bundled sidecar reported version `0.1.0-alpha.0` and the current commit;
- Tauri built a native x64 `Switchyard.app` and 24 MiB DMG; `hdiutil verify`
  reported a valid checksum;
- an ad-hoc signature passed `codesign --verify --deep --strict`;
- the packaged app launched a visible 1440×960 `Switchyard` window and started
  its bundled detached daemon under the expected per-user data directory;
- clicking the standard close control removed the window while the desktop
  process remained alive, and a second launch restored the same instance;
- the app's Info.plist contains the `switchyard` URL scheme; deep-link parsing
  and route selection passed unit tests; and
- quitting the adapter left the independent daemon running as documented.

The complete repository `make quality` run passed: generated artifacts were
clean; Go formatting, vet, GolangCI-Lint, architecture checks, the full suite,
race detection, migration checks, and Govulncheck passed; all 20 Vue unit tests,
four browser E2E tests, and nine visual tests passed; all six Rust tests,
rustfmt, and Clippy with `-D warnings` passed; and the production Vue/Go build,
native application, and DMG were produced. Apple notarization itself is
credential-gated and is verified by the protected release workflow rather than
a local debug build.

## Known limitations and deferred work

- Phase 15 packaged macOS first. Phase 18 now supplies the Linux/Windows bundle
  matrix, native Windows adapters, and explicit WSL behavior.
- Signing, notarization, and production updater-feed access require external
  Apple and Minisign credentials and cannot be truthfully exercised by an
  unsigned local build.
- The daemon intentionally outlives the desktop adapter so CLI, browser, MCP,
  and detached terminal sessions remain available.
