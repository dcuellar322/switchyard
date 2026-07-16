# Switchyard

Switchyard is a local, project-oriented development command center. A single Go
control plane owns project state and local capabilities; the CLI, browser UI,
Tauri desktop shell, and MCP server are adapters over the same application
services.

Switchyard v1 is implemented from the phased
[implementation plan](SWITCHYARD_IMPLEMENTATION_PLAN.md). Implementation and
verification evidence is recorded under [`docs/progress`](docs/progress/).

Start with the [getting-started guide](docs/getting-started.md), then review
[platform support](docs/platform-support.md), the
[v1 compatibility policy](docs/compatibility.md), and
[security model](docs/security/threat-model.md).

## Architecture at a glance

```text
CLI / HTTP / WebSocket / MCP / Tauri adapters
                      |
                      v
              application use cases
                      |
                      v
                  domain model

infrastructure adapters -- implement --> application ports
```

Switchyard is a modular monolith. Domain packages own their invariants and do
not import transports, databases, Docker, operating-system commands, desktop
concepts, or AI-provider SDKs. Cross-domain work uses explicit application
interfaces or typed events. See the
[architecture overview](docs/architecture/README.md) and
[repository policy](AGENTS.md).

## Process topology

The `switchyard` binary hosts the daemon, CLI, REST/WebSocket API, and MCP
façade. It talks to local infrastructure through focused adapters and stores
durable state in SQLite. The Vue application and thin Tauri shell never own
runtime orchestration rules.

## Project vocabulary

The canonical meanings of project, service, runtime, run, operation, action,
workspace, declaration, reservation, and binding are in the
[product glossary](docs/glossary.md).

## Architecture decisions

Accepted and planned decisions are indexed in [`docs/adr`](docs/adr/README.md).
The foundational accepted ADRs cover the modular monolith, Go control plane,
single binary, REST/WebSocket contracts, SQLite persistence, thin Tauri shell,
MCP-first agents, and platform order.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md) before making
changes. Feature work follows one roadmap phase or one approved vertical slice
at a time and includes tests and progress evidence.

## Development

Prerequisites are Go 1.26.5, Node 22.13 or newer, pnpm 11.13.1, and Rust 1.97.1
for the optional native shell. Toolchain versions are pinned in `.go-version`,
`.node-version`, `rust-toolchain.toml`, `go.mod`, and the root `package.json`.

```bash
make bootstrap
make build
./bin/switchyard --data-dir .switchyard-data/dev doctor
./bin/switchyard --data-dir .switchyard-data/dev project add .
./bin/switchyard --data-dir .switchyard-data/dev project list
```

The CLI starts the local daemon on demand. Run `switchyard ui` to print a
one-time browser URL; direct unauthenticated API requests are rejected. The
[CLI reference](docs/cli.md) documents stable JSON/JSONL output, semantic exit
codes, shell completions, and automation rules. Run `make quality` for the
complete local quality gate or the focused Make targets documented by
`make -n quality`.

On macOS, Linux, and Windows, `make desktop-build` produces native bundles
containing the same Go control plane. `pnpm desktop:dev` runs the shell locally. The shell
adds a tray, native notifications, optional launch-at-login, deep links, and
signed updates without moving product policy into Rust. See the
[desktop installation guide](docs/desktop-installation.md),
[release engineering guide](docs/release.md), and
[desktop architecture](docs/architecture/desktop-shell.md). The standalone Go
binary remains the supported headless/server installation.

External tools can integrate through the versioned, out-of-process plugin SDK
without entering the daemon address space. Packages are discovered without
execution, trusted by exact executable fingerprint, enabled with explicit
scopes, supervised per call, and automatically disabled after identity changes.
See the [plugin SDK and compatibility policy](docs/plugin-sdk.md).

Troubleshooting is deterministic first: Switchyard builds bounded redacted
diagnostic receipts from current project facts, recognizes common failures,
and optionally asks an isolated provider for schema-constrained hypotheses that
must cite local evidence. Suggested actions remain existing accepted actions;
saved recipes are created disabled, rate-limited, inspectable, and audited.
Feedback and deduplicated alerts stay local, and cleanup remains preview-only.
See the [diagnostics and safe automation architecture](docs/architecture/intelligent-diagnostics.md).

Trusted Docker Compose projects support reviewable lifecycle plans, durable
start/stop/restart/pause/rebuild/teardown operations, live status, bounded logs,
and current resource metrics. See the
[Compose runtime architecture](docs/architecture/docker-compose-runtime.md).

Trusted native-process projects support shell-free uv/npm/script/Make-style
commands, dependency-ordered multi-service lifecycle, durable process-tree
fingerprints, crash outcomes, bounded opt-in restart, stdout/stderr capture,
metrics, and honest external-listener recognition. See the
[native process runtime architecture](docs/architecture/native-process-runtime.md).

Both runtimes feed the same redaction-first persistent log archive and health
evaluator. Project diagnostics include required-readiness gates, stale and
disconnected observer states, rotating bounded log history, cursor-resumable
live logs, operation-correlated export, availability-aware metric history,
sustained resource budgets, and honest Docker storage attribution. Cleanup is
preview-only and exposes no automatic deletion capability. See the
[observability architecture](docs/architecture/observability.md).

The developer workflow layer combines provenance-bearing declared, reserved,
and bound ports; preferred-range suggestions; fresh porcelain-v2 Git state;
and durable, risk-classified project actions. Built-in terminal, VS Code,
Codex, Claude Code, Git, and endpoint actions remain confined to trusted roots
and produce redaction-safe audits. See the
[developer workflows architecture](docs/architecture/developer-workflows.md).

The browser dashboard provides the responsive project command center, bounded
logs, retained resource charts, exact storage inventory/preview, operation
progress, keyboard command palette, and honest partial or Docker-disconnected
states. It remains a generated-contract adapter and never constructs runtime
commands. See the
[dashboard alpha architecture](docs/architecture/dashboard-alpha.md).

Coding agents use a permission-scoped MCP stdio adapter over the same daemon
use cases. The default observe profile cannot mutate, destructive tools require
an explicit admin profile, responses are bounded and redacted, and no generic
shell tool exists. Install project-local Codex or Claude Code configuration
with `switchyard agent install codex` or `switchyard agent install claude`.
See the [MCP reference](docs/mcp.md) and
[agent integration architecture](docs/architecture/agent-integration.md).

Ambiguous onboarding can optionally use Codex CLI, Claude Code, or one
explicitly configured OpenAI-compatible endpoint. Switchyard previews and
persists the exact redacted evidence payload, runs CLI providers outside the
repository, schema-validates and evidence-checks every suggestion, retains
high-confidence deterministic facts, and requires human approval of the
resulting untrusted revision. Provider absence never affects deterministic
onboarding. See the
[AI-assisted onboarding architecture](docs/architecture/ai-assisted-onboarding.md).

Workspaces coordinate trusted projects and registered Git worktrees as a
validated dependency graph. Starts respect health gates and failure policy,
stops preserve data unless removal is explicitly confirmed, and every
worktree receives stable Compose, port-lease, and optional `.localhost`
routing identity. See the
[workspace and worktree architecture](docs/architecture/workspaces.md).

Trusted projects can also launch embedded project, service, database, Codex,
Claude Code, and reviewed interactive-action PTYs. Browser disconnects detach
for a documented 30-minute idle window, reconnect output is memory-bounded,
SQLite retains metadata only, and external terminal handoff remains available.
Agent records describe user-visible terminal output and never claim access to
hidden reasoning. See the
[terminal and agent session architecture](docs/architecture/terminal-sessions.md).

Optional peer federation connects explicitly configured Switchyard daemons
over an operator-owned tunnel with mutual TLS, exact certificate pins, separate
capability grants, bounded inventory, typed lifecycle operations, and durable
audit. It remains disabled by default and local projects require no account or
cloud service. See the [federation guide](docs/federation.md).

## Security

Switchyard treats repositories, local processes, Docker, browser clients,
coding agents, AI providers, and plugins as separate trust boundaries. See
[SECURITY.md](SECURITY.md) for reporting and baseline security rules.
Switchyard collects no required telemetry; optional provider and support-bundle
behavior is documented in the [privacy statement](docs/privacy.md). Usage help
and report boundaries are in [SUPPORT.md](SUPPORT.md).

## License

Apache-2.0. See [LICENSE](LICENSE).
