---
title: "ADR-0016: Optional peer federation and signed shared configuration"
description: Extend beyond one machine without weakening local-only defaults or ambient authority.
category: concept
audience: [integrator, contributor, maintainer]
lastVerified: 2026-07-17
---

- Status: Accepted
- Date: 2026-07-16

## Context

Switchyard v1 deliberately trusts one local user and one local control plane.
Post-v1 demand adds remote development machines, encrypted configuration sync,
team templates, curated plugin metadata, and enterprise policy. Extending the
loopback API over a network would leak ambient local authority, while requiring
a Switchyard cloud service would break the local-only product and deployment
model.

## Decision

Add optional peer federation without changing the local control plane's
defaults. A daemon may expose a separate `switchyard.remote/v1` agent listener
only when explicitly configured with a server certificate, private key, client
CA, and controller allowlist. The listener requires mutual TLS, pins the
authenticated controller certificate fingerprint, and offers a narrow typed
protocol for identity, bounded inventory, and reviewed project lifecycle
operations. It never exposes the browser session API, terminal streams, raw
logs, secrets, Docker, SQL, plugin processes, MCP, or a generic shell.

The controlling daemon stores machine identity, tunnel HTTPS endpoint,
certificate pin, capabilities, grants, and credential references. It does not
store private credential values. Operators supply the tunnel layer (for
example WireGuard, Tailscale, or an SSH forward); Switchyard authenticates and
authorizes both ends independently of tunnel routing. Remote mutations require
an enabled machine, an explicitly granted capability, a matching peer
identity, confirmation for the declared risk, and durable audit on both peers.

Shared configuration uses canonical `switchyard.bundle/v1` envelopes. Team
templates, policy packs, curated plugin registries, and enterprise
configuration are signed with Ed25519 and verified against explicitly trusted
publishers before installation. Portable sync archives are encrypted to an
explicit age recipient before leaving a machine. Import is previewable,
conflict-aware, and never includes secret values, repository contents, logs,
terminal output, or runtime state.

Anonymous usage metrics remain disabled by default. Enabling them requires an
explicit HTTPS endpoint and displays the exact bounded aggregate fields.
Existing local workflows never depend on a remote peer, bundle publisher,
telemetry endpoint, or cloud account. The browser provides a responsive
read-only companion view over the controller's already authenticated local
session; it does not create a public web service.

## Consequences

Local-only mode remains the zero-configuration path and all federation code is
behind application ports. Machine outages degrade only remote inventory.
Certificates, signing keys, age identities, and tunnel setup become explicit
operator responsibilities with rotation and revocation procedures.

The direct peer model does not provide rendezvous, NAT traversal, hosted
identity, organization billing, or fleet-wide secret distribution. Those
would be separate products and require new decisions. Remote terminal access
and arbitrary command execution remain intentionally absent.
