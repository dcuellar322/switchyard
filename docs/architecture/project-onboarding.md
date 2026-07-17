---
title: Project onboarding and manifest resolution
description: Safe discovery, evidence aggregation, trust, validation, precedence, and provenance.
category: concept
audience: [user, contributor, integrator]
since: 1.0.0
lastVerified: 2026-07-17
---

Phase 3 introduces the catalog, deterministic discovery, and canonical project
manifest. Repository content remains untrusted until a person accepts a
proposal.

## Trust boundary

```text
user-selected directory
        |
        v
canonical root + containment checks
        |
        v
fixed-file scanners (read only, <= 1 MiB)
        |
        v
evidence with source path and line range
        |
        v
deterministic proposal + validation
        |
        v
human approval -> trusted immutable snapshot
```

Scanning never executes repository commands. It does not read `.env`, private
keys, credentials, arbitrary home-directory files, or symlinks that escape the
selected root. The Compose scanner decodes service and port declarations but
does not return environment values. The Node scanner proposes package-manager
commands from script names rather than copying script bodies. README evidence
is limited to the first title and credential-like text is redacted.

Each scanner is an independent evidence producer. The proposal builder has no
filesystem access and combines evidence using stable rules. Missing or
ambiguous runtime facts remain in `unresolved`; unresolved proposals cannot be
accepted.

## Durable review model

A scan atomically creates a pending project, manifest proposal, and normalized
evidence rows. Repeating `add` for the same canonical root returns the existing
proposal instead of creating competing registrations. Approval uses a
compare-and-swap status transition and one SQLite transaction to:

1. accept the proposal;
2. supersede an older accepted proposal, if present;
3. append an immutable manifest snapshot;
4. increment the project manifest revision;
5. change the project trust state to `trusted`; and
6. append a mutation audit event.

No repository file is written by scanning or approval.

## Canonical contract and precedence

`internal/manifest/domain.Manifest` is the source of truth for the portable
manifest. `tools/schema-gen` generates and commits
`internal/manifest/schema/project.schema.json`; generated-artifact checks fail
when it drifts. YAML decoding rejects unknown fields and validation applies the
schema, domain invariants, canonical path containment, executable availability,
port rules, and health-check syntax.

Effective values merge in this order, with the later source winning:

1. live deterministic discovery;
2. accepted inference stored in SQLite;
3. `.switchyard/project.yml`;
4. `.switchyard/project.local.yml`; and
5. a runtime override for the current operation.

The resolver retains the winning source for every leaf as a JSON Pointer.
`switchyard manifest explain`, `diff`, and `validate` expose the result through
the same generated local API used by the UI.
