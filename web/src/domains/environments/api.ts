import {
  listLocalRoutes,
  listProjectEnvironments,
  registerProjectEnvironments,
  updateEnvironment,
} from '../../api/generated/sdk.gen'
import type { EnvironmentRegistration, LocalRoute, Project, ProjectEnvironment } from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

function headers(): { 'Idempotency-Key': string } {
  return mutationHeaders(`ui_${crypto.randomUUID()}`) as { 'Idempotency-Key': string }
}

export async function loadProjectEnvironments(projectId: string): Promise<Array<ProjectEnvironment>> {
  const result = await listProjectEnvironments({ path: { projectId } })
  if (result.error || !result.data) throw new Error('Project environments are unavailable.')
  return result.data
}

export async function loadAllEnvironments(projects: Array<Project>): Promise<Array<ProjectEnvironment>> {
  const results = await Promise.all(projects.map((project) => loadProjectEnvironments(project.id)))
  return results.flat()
}

export async function registerEnvironments(projectId: string): Promise<EnvironmentRegistration> {
  const result = await registerProjectEnvironments({ path: { projectId }, headers: headers() })
  if (result.error || !result.data) throw new Error('Git worktrees could not be registered.')
  return result.data
}

export async function renameEnvironment(environmentId: string, hostname: string): Promise<ProjectEnvironment> {
  const result = await updateEnvironment({ path: { environmentId }, body: { hostname }, headers: headers() })
  if (result.error || !result.data) throw new Error('The environment hostname could not be updated.')
  return result.data
}

export async function loadRoutes(): Promise<Array<LocalRoute>> {
  const result = await listLocalRoutes()
  if (result.error || !result.data) throw new Error('Local routes are unavailable.')
  return result.data
}
