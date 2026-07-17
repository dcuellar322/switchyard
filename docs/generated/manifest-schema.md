---
title: Generated project manifest schema index
description: Top-level project manifest fields generated from the committed canonical JSON Schema.
category: reference
audience: [user, integrator, contributor]
since: 1.0.0
lastVerified: 2026-07-17
---

Source schema: [`https://switchyard.dev/schema/project.v1.json`](/docs/manifest-reference/).
Do not edit this generated page manually.

| Field | Shape | Required | Description |
|---|---|:---:|---|
| `schemaVersion` | string | yes |  |
| `kind` | string | yes |  |
| `metadata` | Metadata | yes |  |
| `repository` | Repository | yes |  |
| `runtime` | Runtime | no |  |
| `lifecycle` | Lifecycle | no |  |
| `services` | array | no |  |
| `ports` | array | no |  |
| `endpoints` | array | no |  |
| `actions` | array | no |  |
| `resourcePolicy` | ResourcePolicy | no |  |
