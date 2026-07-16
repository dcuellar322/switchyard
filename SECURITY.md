# Security policy

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Until a private
security contact is published, use GitHub's private vulnerability reporting for
this repository. Include affected versions, impact, reproduction steps, and any
suggested mitigation. Maintainers will acknowledge a complete report as soon as
practical and coordinate disclosure after a fix is available.

## Supported versions

Switchyard has not released a stable version. Security fixes apply to the
current development branch until release channels are published.

## Security baseline

- Repositories are untrusted until explicitly approved.
- Deterministic discovery reads bounded, allowlisted files and never executes
  repository commands.
- Process execution uses argument arrays, explicit working directories,
  cancellation, and risk classification; shell interpretation is opt-in.
- Browser access is loopback-only, authenticated, CSRF-protected, and subject
  to WebSocket origin checks.
- Secrets live in the operating-system keychain and are redacted before logs,
  exports, diagnostics, or provider requests.
- Agent and plugin capabilities are least-privilege, permission-checked, and
  audited. Switchyard does not expose a generic shell MCP tool.
- Destructive operations require preview and explicit authorization.

The complete trust model is normative in Section 18 of
`SWITCHYARD_IMPLEMENTATION_PLAN.md`.
