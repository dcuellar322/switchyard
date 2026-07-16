# Product glossary

These terms have one canonical meaning across domain code, APIs, UI, CLI, MCP,
documentation, and tests.

| Term | Meaning |
|---|---|
| Project | A registered development repository/location and its effective operational definition. It is the product's top-level entity. |
| Service | A project-visible runtime unit, such as a Compose service, managed host process, or observed external service. |
| Runtime | The mechanism and configuration used to inspect and operate services, such as Docker Compose or native processes. |
| Run | One time-bounded execution of a project or service, with origin, driver, identity, start/end, and outcome. |
| Operation | A durable requested unit of work with risk, requester, progress, cancellation, audit, and terminal state. |
| Action | A declared, typed, risk-classified developer command or launch behavior that may create an operation. |
| Workspace | A dependency graph of related projects/environments with coordinated lifecycle behavior. |
| Declaration | Configuration evidence that a service expects a host port. It does not prove the port is reserved or listening. |
| Reservation | A durable user/system claim protecting a port while no service is necessarily listening. |
| Lease | A reservation with an owner and lifecycle, commonly assigned to a worktree or generated environment. |
| Binding | An observed operating-system or Docker listener currently using an address, protocol, and port. |
| Manifest | The versioned portable project definition plus any machine-local overlay, resolved with provenance. |
| Proposal | An untrusted, reviewable candidate manifest built from deterministic evidence and optionally AI evidence. |
| Driver | An adapter implementing inspection, planning, lifecycle execution, logs, and metrics for one runtime kind. |
| Reconciliation | Deriving current project/service state by comparing declared intent, recorded ownership, and live observations. |
| Trust state | The repository authorization state: `untrusted`, `review_pending`, `trusted`, or `blocked`. |
| Teardown | A destructive lifecycle operation that removes runtime resources according to an explicit preview; it is not synonymous with stop. |
| Pause | Suspending runtime execution while retaining runtime state; only exposed when the driver can do so safely. |
| Evidence | A bounded fact with source, source location, confidence, warnings, and structured data. |

Display names are never identifiers. IDs are stable; slugs are human-friendly
selectors and must be unique where used.
