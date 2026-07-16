# Switchyard

Switchyard is a local, project-oriented development command center. A single Go
control plane owns project state and local capabilities; the CLI, browser UI,
Tauri desktop shell, and MCP server are adapters over the same application
services.

The repository is being built from the phased
[implementation plan](SWITCHYARD_IMPLEMENTATION_PLAN.md). Current implementation
evidence is recorded under [`docs/progress`](docs/progress/).

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

Prerequisites are Go 1.26.5, Node 22.13 or newer, and pnpm 11.13.1. Toolchain
versions are pinned in `.go-version`, `.node-version`, `go.mod`, and the root
`package.json`.

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
live logs, and operation-correlated export. See the
[observability architecture](docs/architecture/observability.md).

The developer workflow layer combines provenance-bearing declared, reserved,
and bound ports; preferred-range suggestions; fresh porcelain-v2 Git state;
and durable, risk-classified project actions. Built-in terminal, VS Code,
Codex, Claude Code, Git, and endpoint actions remain confined to trusted roots
and produce redaction-safe audits. See the
[developer workflows architecture](docs/architecture/developer-workflows.md).

The browser dashboard provides the responsive project command center, bounded
logs and resources, operation progress, keyboard command palette, and honest
partial or Docker-disconnected states. It remains a generated-contract adapter
and never constructs runtime commands. See the
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

## Security

Switchyard treats repositories, local processes, Docker, browser clients,
coding agents, AI providers, and plugins as separate trust boundaries. See
[SECURITY.md](SECURITY.md) for reporting and baseline security rules.

## License

Apache-2.0. See [LICENSE](LICENSE).
