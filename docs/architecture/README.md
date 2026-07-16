# Architecture overview

Switchyard is a local-first modular monolith delivered primarily as one Go
binary. The process hosts a daemon, CLI commands, local HTTP/WebSocket and IPC
transports, and an MCP façade. A Vue application and thin Tauri desktop shell
consume the same application behavior.

The [durable operations and local transport](operations-kernel.md) note defines
the command state machine, restart behavior, event replay, Unix IPC, and browser
session handshake introduced by Phase 2.

The [project onboarding and manifest resolution](project-onboarding.md) note
defines the Phase 3 repository trust boundary, evidence pipeline, approval
transaction, generated schema, and configuration precedence.

The [Docker Compose runtime](docker-compose-runtime.md) and
[native process runtime](native-process-runtime.md) notes describe the two
initial lifecycle drivers, their evidence models, and their ownership limits.
The [health and log observability](observability.md) note defines the redaction
boundary, rotating storage, cursor replay, health scheduling, and degraded-state
derivation introduced by Phase 7.
The [ports, source control, and trusted actions](developer-workflows.md) note
defines Phase 8 port provenance and reservations, read-only Git observation,
action authorization, working-directory containment, and audit recovery.

## Process topology

```text
CLI          Browser/Vue          Tauri          MCP clients
 |                |                 |                 |
 +---------- local IPC / loopback HTTP / stdio -------+
                              |
                   transport adapters
                              |
                   application use cases
                              |
                       domain modules
                              |
          focused infrastructure adapter interfaces
               /        |        |        \
           SQLite     Docker   processes   Git/OS
```

The daemon owns project truth. Clients never call Docker, SQL, or operating
system process APIs directly. Tauri/Rust contains packaging and native-shell
integration only.

## Dependency direction

```text
transport and UI adapters -> application -> domain
infrastructure adapters ----- implement ---> application ports
```

A domain owns its model, invariants, use cases, persistence interfaces, and
adapter contracts. Domain code must not import transport, database, Docker,
operating-system command, desktop, or AI-provider packages. A domain cannot
read another domain's tables. Cross-domain behavior uses an explicit
application interface or a typed event.

## Bounded contexts

| Context | Responsibility |
|---|---|
| `catalog` | Projects, locations, tags, and trust/approval |
| `manifest` | Configuration, overlays, provenance, proposals, schema versions |
| `operations` | Durable operations, progress, cancellation, serialization |
| `runtime` | Driver contracts, runs, reconciliation, lifecycle execution |
| `observability` | Logs, health, events, metrics, and retention |
| `ports` | Declarations, reservations, bindings, conflicts, and leases |
| `sourcecontrol` | Git repository facts and worktrees |
| `actions` | Typed, risk-classified project actions |
| `workspace` | Multi-project graphs and coordinated lifecycle |
| `agents` | MCP, provider adapters, proposals, sessions, and agent audit |
| `platform` | OS capabilities without product policy |

`foundation` is limited to stable primitives such as clocks, IDs, event
envelopes, pagination, and problem details. It is not a miscellaneous shared
code layer. Root packages named `utils`, `common`, or `helpers` are forbidden.

## Composition

Construct dependencies explicitly at a small composition root. Interfaces live
with their consumer and exist only for a real boundary, alternate adapter, or
test seam. Long-running work receives a context, has defined cancellation, and
records progress and outcome. Architecture checks enforce dependency rules.

Accepted decisions and their consequences are indexed in
[`docs/adr`](../adr/README.md).
