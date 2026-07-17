---
title: Generated settings field index
description: Durable non-secret settings fields generated from the canonical Go domain structs.
category: reference
audience: [user, integrator, contributor]
since: 1.0.0
lastVerified: 2026-07-17
---

The domain model deliberately contains credential references, never credential
values. Do not edit this generated page manually.

| JSON field | Go shape | Domain field |
|---|---|---|
| `rangeStart` | `int` | `RangeStart` |
| `rangeEnd` | `int` | `RangeEnd` |
| `excluded` | `[]int` | `Excluded` |
| `logAgeSeconds` | `int64` | `LogAgeSeconds` |
| `logMaximumBytes` | `int64` | `LogMaximumBytes` |
| `metricRawSeconds` | `int64` | `MetricRawSeconds` |
| `metricMinuteSeconds` | `int64` | `MetricMinuteSeconds` |
| `metricQuarterHourSeconds` | `int64` | `MetricQuarterHourSeconds` |
| `maximumMetricHistoryPoints` | `int` | `MaximumMetricHistoryPoints` |
| `terminal` | `string` | `Terminal` |
| `editor` | `string` | `Editor` |
| `id` | `string` | `ID` |
| `enabled` | `bool` | `Enabled` |
| `executable` | `string` | `Executable` |
| `endpoint` | `string` | `Endpoint` |
| `model` | `string` | `Model` |
| `credentialReference` | `string` | `CredentialReference` |
| `defaultProvider` | `string` | `DefaultProvider` |
| `providers` | `[]ProviderPreferences` | `Providers` |
| `defaultAgentProfile` | `string` | `DefaultAgentProfile` |
| `density` | `string` | `Density` |
| `timeDisplay` | `string` | `TimeDisplay` |
| `theme` | `string` | `Theme` |
| `revision` | `int64` | `Revision` |
| `projectRoots` | `[]string` | `ProjectRoots` |
| `ports` | `PortPreferences` | `Ports` |
| `retention` | `RetentionPreferences` | `Retention` |
| `tools` | `ToolPreferences` | `Tools` |
| `ai` | `AIPreferences` | `AI` |
| `permissions` | `PermissionPreferences` | `Permissions` |
| `appearance` | `AppearancePreferences` | `Appearance` |
| `updatedAt` | `time.Time` | `UpdatedAt` |
