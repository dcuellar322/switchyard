---
title: Optional peer federation
description: Configure explicit post-v1 peer inventory and reviewed remote lifecycle operations.
category: concept
audience: [user, integrator, maintainer]
since: 1.0.0
lastVerified: 2026-07-17
---

Switchyard can connect directly to another Switchyard daemon for bounded
machine inventory and typed lifecycle operations. Federation is disabled by
default. It introduces no account, hosted control plane, or dependency for
local projects.

## Trust and transport model

An operator supplies the network path, such as a WireGuard or Tailscale link or
an SSH local forward. Switchyard does not configure or trust that tunnel. The
remote-agent listener independently requires TLS 1.3 mutual authentication,
and both sides pin identity:

- the agent verifies the controller certificate against an explicit client CA;
- the agent hashes the verified leaf certificate and applies an explicit
  application capability allowlist;
- the controller verifies the peer CA and exact reviewed leaf-certificate
  SHA-256 fingerprint;
- the controller separately grants only capabilities declared by that peer;
- every accepted or denied remote mutation records controller identity,
  machine identity, request identity, capability outcome, and operation ID.

The remote protocol contains identity, project/environment summaries, and
`start`, `stop`, `restart`, or `rebuild`. It has no terminal, generic shell,
SQL, Docker, log, secret, source-file, plugin, or MCP endpoint. Inventory omits
repository locations, runtime-native identifiers, Git changes, environment
values, logs, and terminal output.

## Agent setup

Create a private CA and distinct server and client certificates with the
appropriate server-auth and client-auth usages. Keep private keys outside the
repository with owner-only permissions. Record the SHA-256 fingerprint of each
leaf certificate through an independent channel.

Run the peer daemon explicitly:

```text
switchyard daemon \
  --remote-address 127.0.0.1:19618 \
  --remote-tls-certificate /absolute/pki/agent.pem \
  --remote-tls-key /absolute/pki/agent-key.pem \
  --remote-client-ca /absolute/pki/controller-ca.pem \
  --remote-machine-id buildbox-01 \
  --remote-machine-name "Build box" \
  --remote-controller <controller-sha256>=inventory.read,project.operate
```

Use a private interface or tunnel endpoint. Binding the agent to a broader
interface does not weaken its mTLS requirement, but it unnecessarily expands
network exposure.

`inventory.read` is mandatory for a controller. Add `project.operate` or
`environment.manage` only when that controller needs the corresponding typed
mutation. Restart the daemon to change its controller allowlist; configuration
is never fetched from a cloud service.

## Controller setup

Register the peer through the Machines view or CLI:

```text
switchyard machine add "Build box" https://127.0.0.1:19618 \
  --server-fingerprint <agent-sha256> \
  --ca /absolute/pki/agent-ca.pem \
  --client-certificate /absolute/pki/controller.pem \
  --client-key /absolute/pki/controller-key.pem \
  --grant inventory.read \
  --yes
```

Registration performs an authenticated probe. The private key value is never
placed in SQLite or an API response; the database retains only its local file
reference. Review additional access separately:

```text
switchyard machine access <machine> \
  --grant inventory.read \
  --grant project.operate \
  --yes

switchyard machine snapshot <machine>
switchyard machine run <machine> <remote-project-id> restart --yes
switchyard machine disable <machine> --yes
```

`machine remove` deletes only the controller registration. It does not contact
or reconfigure the peer, and the local authorization audit is retained.

## Companion view

`/companion` is a responsive, authenticated, read-only rendering of configured
machine and project health. It contains no registration, permission, or
lifecycle controls. The normal same-origin browser session still applies; an
operator who carries it over a private tunnel remains responsible for that
network path.

## Failure behavior

Certificate rotation intentionally fails closed as a pin change. Disable the
machine, verify the new certificate out of band, and create a new reviewed
registration. A failed probe records bounded state without exposing raw peer
responses. Removing the remote listener flags returns the daemon to normal
local-only operation without migrating or changing any project.
