---
title: Generated MCP tool index
description: Tool names, profiles, risks, and descriptions generated from the MCP server registration.
category: reference
audience: [user, integrator, contributor]
since: 1.0.0
lastVerified: 2026-07-17
---

Tools call application services and are omitted when the selected permission
profile cannot use them. Do not edit this generated page manually.

| Tool | Title | Minimum profile | Risk | Purpose |
|---|---|---|---|---|
| `switchyard_action_run` | Run trusted action | develop | declared | Queue one reviewed manifest action; risk confirmation is enforced. |
| `switchyard_actions_list` | List trusted actions | observe | read-only | List bounded trusted project actions and their risk metadata. |
| `switchyard_environments_list` | List project environments | observe | read-only | List registered Git worktrees and exact runtime leases. |
| `switchyard_environments_register` | Register worktree environments | develop | filesystem-read | Reconcile trusted Git worktrees and allocate exact ports without running repository code. |
| `switchyard_manifest_explain` | Explain effective manifest | observe | read-only | Read the effective trusted manifest and field provenance. |
| `switchyard_manifest_proposal_accept` | Accept manifest proposal | admin | trust-decision | Accept a previously validated proposal as an explicit trust decision. |
| `switchyard_manifest_proposal_create` | Create manifest proposal | maintain | filesystem-read | Scan a local repository without executing its code and return an untrusted proposal for review. |
| `switchyard_operation_cancel` | Cancel operation | develop | mutating | Request cooperative cancellation of one durable operation. |
| `switchyard_operation_get` | Get operation | observe | read-only | Read one durable operation and its terminal or cancellation state. |
| `switchyard_operation_wait` | Wait for operation | observe | read-only | Wait up to 30 seconds for an operation and emit MCP progress notifications. |
| `switchyard_ports_list` | List local ports | observe | read-only | Read bounded declarations, leases, listeners, and conflicts. |
| `switchyard_ports_suggest` | Suggest local port | observe | read-only | Find an available port in an explicit bounded range without reserving it. |
| `switchyard_project_get` | Get Switchyard project | observe | read-only | Read one registered project by opaque identifier. |
| `switchyard_project_git_status` | Get project Git status | observe | read-only | Read bounded Git branch, changes, remotes, worktrees, and last commit. |
| `switchyard_project_health_wait` | Wait for project health | observe | read-only | Wait up to 30 seconds for structured project health to become healthy. |
| `switchyard_project_health` | Get project health | observe | read-only | Read structured persisted health diagnostics. |
| `switchyard_project_logs_query` | Query project logs | observe | read-only | Read bounded, redacted recent project logs. |
| `switchyard_project_pause` | Pause project | develop | mutating | Queue an idempotent pause for a project or selected declared services. |
| `switchyard_project_rebuild` | Rebuild project | maintain | mutating | Queue an explicit rebuild for a project or selected declared services. |
| `switchyard_project_restart` | Restart project | develop | mutating | Queue a restart for a project or selected declared services. |
| `switchyard_project_resume` | Resume project | develop | mutating | Queue an idempotent resume for a project or selected declared services. |
| `switchyard_project_services` | List project services | observe | read-only | Read bounded service observations for one project. |
| `switchyard_project_start` | Start project | develop | mutating | Queue an idempotent start for a project or selected declared services. |
| `switchyard_project_status` | Get project status | observe | read-only | Read catalog, runtime, and health status for one project. |
| `switchyard_project_stop` | Stop project | develop | mutating | Queue an idempotent stop for a project or selected declared services. |
| `switchyard_project_teardown` | Tear down project | admin | destructive | Tear down Compose resources; volume removal is explicit and destructive. |
| `switchyard_projects_list` | List Switchyard projects | observe | read-only | List a bounded set of registered projects and trust states. |
| `switchyard_routes_list` | List local routes | observe | read-only | List friendly localhost routes visible to this agent scope. |
| `switchyard_system_info` | Switchyard system info | observe | read-only | Read daemon version, API, schema, and readiness. |
| `switchyard_workspace_get` | Get workspace | observe | read-only | Read one workspace dependency graph and latest execution. |
| `switchyard_workspace_start` | Start workspace | develop | mutating | Queue dependency-ordered workspace start with bounded concurrency. |
| `switchyard_workspace_stop` | Stop workspace | develop | conditional-destructive | Queue dependency-safe workspace stop; data removal is separately authorized. |
| `switchyard_workspaces_list` | List workspaces | observe | read-only | List bounded workspace graphs visible to this agent scope. |
