import {
  acceptManifestProposal,
  createManifestProposal,
  getProjectHealth,
  getProjectLogs,
  getProjectRuntime,
  getProjectGit,
  listProjectActions,
  createActionOperation,
  listProjects,
  validateManifestProposal,
} from '../../api/generated/sdk.gen'
import type {
  AcceptedManifestProposal,
  ManifestProposal,
  Project,
  ProjectHealth,
  RuntimeLogEntry,
  RuntimeObservation,
  GitState,
  ProjectActions,
  Operation,
} from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

function requestKey(): string {
  return `ui_${crypto.randomUUID()}`
}

export async function loadProjects(): Promise<Array<Project>> {
  const result = await listProjects()
  if (result.error || !result.data) throw new Error('The project catalog could not be loaded.')
  return result.data
}

export async function scanRepository(path: string): Promise<ManifestProposal> {
  const result = await createManifestProposal({
    body: { path },
    headers: mutationHeaders(requestKey()) as { 'Idempotency-Key': string },
  })
  if (result.error || !result.data) throw new Error('The repository scan could not create a proposal.')
  return result.data
}

export async function revalidateProposal(proposalId: string): Promise<ManifestProposal> {
  const result = await validateManifestProposal({
    path: { proposalId },
    headers: mutationHeaders(requestKey()) as { 'Idempotency-Key': string },
  })
  if (result.error || !result.data) throw new Error('The manifest proposal could not be validated.')
  return result.data
}

export async function approveProposal(proposalId: string): Promise<AcceptedManifestProposal> {
  const result = await acceptManifestProposal({
    path: { proposalId },
    headers: mutationHeaders(requestKey()) as { 'Idempotency-Key': string },
  })
  if (result.error || !result.data) throw new Error('The proposal could not be approved.')
  return result.data
}

export async function loadProjectRuntime(projectId: string): Promise<RuntimeObservation> {
  const result = await getProjectRuntime({ path: { projectId } })
  if (result.error || !result.data) throw new Error('Runtime observation is unavailable.')
  return result.data
}

export async function loadProjectHealth(projectId: string): Promise<ProjectHealth> {
  const result = await getProjectHealth({ path: { projectId } })
  if (result.error || !result.data) throw new Error('Health diagnostics are unavailable.')
  return result.data
}

export async function loadProjectLogs(projectId: string): Promise<Array<RuntimeLogEntry>> {
  const result = await getProjectLogs({ path: { projectId }, query: { tail: 200 } })
  if (result.error || !result.data) throw new Error('Persisted logs are unavailable.')
  return result.data
}

export async function loadProjectGit(projectId: string): Promise<GitState> {
  const result = await getProjectGit({ path: { projectId } })
  if (result.error || !result.data) throw new Error('Git state is unavailable.')
  return result.data
}

export async function loadProjectActions(projectId: string): Promise<ProjectActions> {
  const result = await listProjectActions({ path: { projectId } })
  if (result.error || !result.data) throw new Error('Project actions are unavailable.')
  return result.data
}

export async function runProjectAction(projectId: string, actionId: string, confirmRisk = false): Promise<Operation> {
  const result = await createActionOperation({
    path: { projectId, actionId }, body: { confirmRisk, allowOutsideRoot: false },
    headers: mutationHeaders(requestKey()) as { 'Idempotency-Key': string },
  })
  if (result.error || !result.data) throw new Error('The project action could not be queued.')
  return result.data
}
