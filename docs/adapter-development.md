# Adapter development guide

Add operating-system or external-tool behavior behind an application-owned
interface. Keep domains platform-neutral and preserve the dependency direction
adapter/transport to application to domain. Do not add OS branches to domain
models, a service locator, or a generic command helper.

An adapter change includes capability detection, typed unavailable/degraded
errors, argument-array execution, cancellation, bounded output, redaction where
data crosses a sink, and tests for success, failure, cancellation, permissions,
and reconciliation. Platform files use explicit Go build tags and must compile
in the Linux and Windows `platform-check` target. Native behavior also runs on
its host in CI.

Process adapters must preserve fingerprint ownership and whole-tree stop.
Port adapters return facts and provenance rather than inferred ownership.
Terminal adapters implement resize, Unicode/ANSI transport, bounded detach,
tree termination, and user-visible output only. Launchers accept only typed
terminal/editor/browser operations; browser targets remain HTTP(S).

Plugin adapters belong out of process and use the public `sdk/plugin` protocol
plus conformance kit. Provider-specific AI code stays under
`internal/agents/providers`; provider output remains untrusted schema input.

Update an ADR before changing an accepted boundary, generate contracts rather
than editing generated files, document the platform matrix, and attach progress
evidence for the phase or vertical slice.
