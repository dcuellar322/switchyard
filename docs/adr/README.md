---
title: Architecture decision records
description: Accepted technical decisions and the process for superseding them.
category: contributor
audience: [contributor, maintainer, integrator]
lastVerified: 2026-07-17
slug: docs/adr
---

ADRs use the status values `Proposed`, `Accepted`, `Superseded`, or `Rejected`.
Changing an accepted decision requires a new ADR that links to and supersedes
the old one.

Section 22's Phase 0 shorthand numbers six foundational decisions differently
from the canonical list in Section 26 of the implementation plan. This index
uses Section 26 numbering because it is the complete, collision-free list. The
Phase 0 subjects remain covered by the accepted records below.

## Accepted foundational decisions

- [ADR-0001: Modular monolith and dependency direction](0001-modular-monolith.md)
- [ADR-0002: Go control plane](0002-go-control-plane.md)
- [ADR-0003: One Switchyard binary](0003-one-binary.md)
- [ADR-0004: REST, WebSocket, and OpenAPI](0004-rest-websocket-openapi.md)
- [ADR-0005: SQLite, sqlc, migrations, and log segments](0005-sqlite-persistence.md)
- [ADR-0006: Docker SDK observation and Compose CLI lifecycle](0006-docker-compose-runtime.md)
- [ADR-0007: Native process ownership and reconciliation](0007-process-ownership.md)
- [ADR-0008: Manifest precedence and provenance](0008-manifest-precedence.md)
- [ADR-0009: Thin Tauri desktop shell](0009-thin-tauri-shell.md)
- [ADR-0010: MCP-first provider-neutral agent integration](0010-mcp-first-agents.md)
- [ADR-0011: Agent permissions and no generic shell tool](0011-agent-permissions.md)
- [ADR-0012: Out-of-process plugin protocol](0012-out-of-process-plugins.md)
- [ADR-0013: Local IPC and browser session security](0013-local-transport-security.md)
- [ADR-0014: Log retention and redaction](0014-log-retention-redaction.md)
- [ADR-0015: Platform support order](0015-platform-order.md)
- [ADR-0016: Optional peer federation and signed shared configuration](0016-optional-federation.md)
- [ADR-0017: Static public site and documentation portal](0017-static-public-site.md)
