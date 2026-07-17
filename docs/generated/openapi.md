---
title: Generated local REST API index
description: Versioned local control-plane operations generated from the canonical OpenAPI contract.
category: reference
audience: [integrator, contributor]
since: 1.0.0
lastVerified: 2026-07-17
---

The browser, desktop, and local clients use this contract through authenticated
local transports. This is not a public network API. Do not edit this generated
page manually.

| Method | Path | Operation ID |
|---|---|---|
| `GET` | `/api/v1/system` | `getSystem` |
| `GET` | `/api/v1/host` | `getHost` |
| `GET` | `/api/v1/settings` | `getDaemonSettings` |
| `PUT` | `/api/v1/settings` | `updateDaemonSettings` |
| `GET` | `/api/v1/resources` | `getResourceOverview` |
| `GET` | `/api/v1/resources/storage` | `getStorageInventory` |
| `GET` | `/api/v1/resources/cleanup-preview` | `getCleanupPreview` |
| `POST` | `/api/v1/auth/bootstrap-tokens` | `createBrowserBootstrapToken` |
| `POST` | `/api/v1/auth/sessions` | `createBrowserSession` |
| `GET` | `/api/v1/operations/{operationId}` | `getOperation` |
| `GET` | `/api/v1/operations` | `listOperations` |
| `POST` | `/api/v1/operations/{operationId}/cancel` | `cancelOperation` |
| `POST` | `/api/v1/manifest-proposals` | `createManifestProposal` |
| `GET` | `/api/v1/manifest-proposals/{proposalId}` | `getManifestProposal` |
| `POST` | `/api/v1/manifest-proposals/{proposalId}/validate` | `validateManifestProposal` |
| `POST` | `/api/v1/manifest-proposals/{proposalId}/accept` | `acceptManifestProposal` |
| `GET` | `/api/v1/ai-providers` | `listAIProposalProviders` |
| `POST` | `/api/v1/manifest-proposals/{proposalId}/ai-preview` | `previewAIManifestEvidence` |
| `POST` | `/api/v1/manifest-proposals/{proposalId}/ai-enhancements` | `createAIManifestEnhancement` |
| `GET` | `/api/v1/manifest-proposals/{proposalId}/ai-enhancements/{operationId}` | `getAIManifestEnhancement` |
| `GET` | `/api/v1/projects` | `listProjects` |
| `GET` | `/api/v1/projects/{projectId}` | `getProject` |
| `DELETE` | `/api/v1/projects/{projectId}` | `removeProject` |
| `POST` | `/api/v1/projects/{projectId}/trust` | `trustProject` |
| `GET` | `/api/v1/projects/{projectId}/manifest/explain` | `explainProjectManifest` |
| `GET` | `/api/v1/projects/{projectId}/manifest/diff` | `diffProjectManifest` |
| `GET` | `/api/v1/projects/{projectId}/manifest/validate` | `validateProjectManifest` |
| `GET` | `/api/v1/projects/{projectId}/runtime` | `getProjectRuntime` |
| `GET` | `/api/v1/projects/{projectId}/health` | `getProjectHealth` |
| `POST` | `/api/v1/projects/{projectId}/runtime/plan` | `planProjectRuntime` |
| `POST` | `/api/v1/projects/{projectId}/operations` | `createProjectOperation` |
| `GET` | `/api/v1/projects/{projectId}/logs` | `getProjectLogs` |
| `GET` | `/api/v1/projects/{projectId}/logs/export` | `exportProjectLogs` |
| `GET` | `/api/v1/projects/{projectId}/metrics` | `getProjectMetrics` |
| `GET` | `/api/v1/projects/{projectId}/metrics/history` | `getMetricHistory` |
| `GET` | `/api/v1/projects/{projectId}/git` | `getProjectGit` |
| `GET` | `/api/v1/projects/{projectId}/environments` | `listProjectEnvironments` |
| `POST` | `/api/v1/projects/{projectId}/environments` | `registerProjectEnvironments` |
| `GET` | `/api/v1/environments/{environmentId}` | `getEnvironment` |
| `PATCH` | `/api/v1/environments/{environmentId}` | `updateEnvironment` |
| `GET` | `/api/v1/routes` | `listLocalRoutes` |
| `GET` | `/api/v1/projects/{projectId}/actions` | `listProjectActions` |
| `POST` | `/api/v1/projects/{projectId}/actions/{actionId}/operations` | `createActionOperation` |
| `GET` | `/api/v1/ports` | `getPortRegistry` |
| `POST` | `/api/v1/ports/suggestions` | `createPortSuggestion` |
| `GET` | `/api/v1/workspaces` | `listWorkspaces` |
| `POST` | `/api/v1/workspaces` | `createWorkspace` |
| `GET` | `/api/v1/workspaces/{workspaceId}` | `getWorkspace` |
| `PUT` | `/api/v1/workspaces/{workspaceId}` | `updateWorkspace` |
| `DELETE` | `/api/v1/workspaces/{workspaceId}` | `deleteWorkspace` |
| `POST` | `/api/v1/workspaces/{workspaceId}/operations` | `createWorkspaceOperation` |
| `GET` | `/api/v1/terminal-sessions` | `listTerminalSessions` |
| `POST` | `/api/v1/terminal-sessions` | `createTerminalSession` |
| `GET` | `/api/v1/terminal-sessions/{terminalSessionId}` | `getTerminalSession` |
| `POST` | `/api/v1/terminal-sessions/{terminalSessionId}/terminate` | `terminateTerminalSession` |
| `GET` | `/api/v1/agents/sessions` | `listAgentSessions` |
| `POST` | `/api/v1/agents/sessions` | `createAgentSession` |
| `GET` | `/api/v1/machines` | `listMachines` |
| `POST` | `/api/v1/machines` | `createMachine` |
| `GET` | `/api/v1/team/publishers` | `listTeamPublishers` |
| `POST` | `/api/v1/team/publishers` | `trustTeamPublisher` |
| `GET` | `/api/v1/team/bundles` | `listTeamBundles` |
| `POST` | `/api/v1/team/bundles` | `installTeamBundle` |
| `POST` | `/api/v1/team/templates/{bundleId}/render` | `renderTeamProjectTemplate` |
| `GET` | `/api/v1/team/policy` | `getEffectiveTeamPolicy` |
| `GET` | `/api/v1/plugin-registry` | `listCuratedPlugins` |
| `GET` | `/api/v1/team/sync` | `exportTeamSync` |
| `POST` | `/api/v1/team/sync/preview` | `previewTeamSync` |
| `POST` | `/api/v1/team/sync/import` | `importTeamSync` |
| `GET` | `/api/v1/telemetry` | `getTelemetryStatus` |
| `PUT` | `/api/v1/telemetry` | `updateTelemetrySettings` |
| `POST` | `/api/v1/telemetry/send` | `sendTelemetryNow` |
| `GET` | `/api/v1/machines/{machineId}` | `getMachine` |
| `DELETE` | `/api/v1/machines/{machineId}` | `deleteMachine` |
| `PUT` | `/api/v1/machines/{machineId}/access` | `updateMachineAccess` |
| `POST` | `/api/v1/machines/{machineId}/probe` | `probeMachine` |
| `GET` | `/api/v1/machines/{machineId}/snapshot` | `getMachineSnapshot` |
| `POST` | `/api/v1/machines/{machineId}/operations` | `createMachineOperation` |
| `GET` | `/api/v1/plugins` | `listPlugins` |
| `POST` | `/api/v1/plugins/refresh` | `refreshPlugins` |
| `POST` | `/api/v1/plugins/{pluginId}/trust` | `trustPlugin` |
| `POST` | `/api/v1/plugins/{pluginId}/enable` | `enablePlugin` |
| `POST` | `/api/v1/plugins/{pluginId}/disable` | `disablePlugin` |
| `POST` | `/api/v1/plugins/{pluginId}/health` | `checkPluginHealth` |
| `GET` | `/api/v1/plugins/{pluginId}/logs` | `listPluginLogs` |
| `POST` | `/api/v1/plugins/{pluginId}/projects/{projectId}/inspection` | `inspectProjectWithPlugin` |
| `POST` | `/api/v1/plugins/{pluginId}/projects/{projectId}/operations` | `createPluginOperation` |
| `GET` | `/api/v1/projects/{projectId}/diagnoses` | `getLatestProjectDiagnosis` |
| `POST` | `/api/v1/projects/{projectId}/diagnoses` | `createProjectDiagnosis` |
| `GET` | `/api/v1/diagnoses/{diagnosisId}` | `getDiagnosis` |
| `POST` | `/api/v1/diagnoses/{diagnosisId}/feedback` | `createDiagnosticFeedback` |
| `POST` | `/api/v1/diagnoses/{diagnosisId}/actions/{actionId}` | `createDiagnosticActionOperation` |
| `GET` | `/api/v1/automation-recipes` | `listAutomationRecipes` |
| `POST` | `/api/v1/automation-recipes` | `createAutomationRecipe` |
| `PATCH` | `/api/v1/automation-recipes/{recipeId}` | `updateAutomationRecipe` |
| `POST` | `/api/v1/projects/{projectId}/automation-evaluations` | `createAutomationEvaluation` |
| `GET` | `/api/v1/diagnostic-notifications` | `listDiagnosticNotifications` |
| `POST` | `/api/v1/diagnostic-notifications/{notificationId}/acknowledgment` | `acknowledgeDiagnosticNotification` |
