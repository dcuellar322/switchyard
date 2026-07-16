# Threat model and v1 security review

This document records the v1 review of Switchyard's local trust boundaries.
The secure default is a single user operating approved local repositories;
repository content, browser pages, processes, containers, plugins, agents, and
AI output are not trusted merely because they are local.

## Assets and boundaries

- Durable project state, manifests, operation/audit history, log segments, and
  desktop preferences.
- Credentials referenced through Keychain, Secret Service, or Credential
  Manager; secret values must remain memory-only and redaction-aware.
- Authority to start/stop process trees and Compose projects, launch reviewed
  actions, allocate ports, and mutate workspaces.
- Unix sockets or Windows named pipes for privileged clients; loopback HTTP for
  an authenticated same-origin browser session.
- External executables: Docker/Compose, Git, shells, editors, coding agents,
  AI-provider CLIs, and explicitly trusted out-of-process plugins.
- Release artifacts, updater metadata, signing keys, and generated contracts.

## Threats and controls

| Threat | Primary controls | Residual risk |
|---|---|---|
| Malicious repository executes during discovery | bounded allowlisted reads; no discovery commands; explicit trust before lifecycle/action/agent operations | an approved repository may intentionally define powerful reviewed actions |
| Cross-origin browser mutation | loopback binding, bootstrap exchange, HttpOnly same-origin cookie, CSRF token, Origin checks for HTTP/WebSocket, no permissive CORS | malware running as the same OS user remains outside the browser threat boundary |
| Another local user invokes privileged API | mode `0600` Unix socket; Windows named-pipe DACL grants only LocalSystem and current-user SID | administrators can inspect or control a user's processes by OS design |
| PID reuse or orphaned child escapes lifecycle | executable/start/cwd fingerprint, process groups on Unix, Job Objects with kill-on-close on Windows, bounded graceful-to-force stop | externally started processes remain observed rather than silently adopted |
| Terminal becomes generic browser shell | server-resolved trusted launch plans, owner-scoped session attachment, bounded input/output, memory-only scrollback, audit metadata | terminal output can contain application data and is visible to its authorized owner |
| Secret reaches logs/provider/support bundle | credential references, redaction before every sink, bounded diagnostic evidence, preview, no source by default | novel secret formats need an explicit redaction pattern |
| Plugin or agent exceeds permission | exact protocol, executable fingerprint trust, capability/scope intersection, per-project profiles, typed tools, audit, no generic MCP shell | a deliberately granted write scope carries the documented capability risk |
| Prompt/log injection changes authority | evidence treated as inert untrusted text, schema validation, evidence citations, deterministic-first results, human review | a provider can return poor advice but cannot authorize an action |
| Update or artifact tampering | protected release environment, platform signing, updater signatures, SHA-256 checksums, keyless Sigstore bundle, SBOM, GitHub artifact attestation | signing-account compromise requires key rotation and release revocation |
| Migration corrupts or strands data | single-daemon lock, compatibility preflight, quick-check, non-overwriting consistent backup, post-backup verification, newer-schema refusal | disk loss affecting both source and same-disk backup requires external backups |

## Review findings

The v1 review found and closed four release-blocking gaps: Windows IPC had no
implementation, Windows child ownership lacked Job Objects, Windows terminals
lacked ConPTY, and OS port inspection depended on macOS-oriented `lsof`. The v1
adapters now use an owner-only named pipe, Job Objects, ConPTY, and portable OS
connection APIs. Browser URL launchers reject non-HTTP(S) targets, and migration
now fails closed around verified backups.

No unresolved critical or high-severity finding remains in the reviewed v1
scope. Accepted medium residual risks are explicit user grants to trusted
plugins/actions, same-user host compromise, and optional provider disclosure
of a previewed redacted evidence bundle. They are product trust decisions, not
hidden defaults.

CI runs CodeQL, dependency review, secret scanning, `govulncheck`, lint,
architecture checks, race tests, strict contract generation, plugin
conformance, browser security tests, and platform adapter tests. A release is
blocked when any required check or signing/attestation step fails.
