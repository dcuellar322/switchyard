import {
  createWorkspace,
  createWorkspaceOperation,
  deleteWorkspace,
  getWorkspace,
  listWorkspaces,
  updateWorkspace,
} from '../../api/generated/sdk.gen'
import type {
  Operation,
  Workspace,
  WorkspaceDefinition,
  WorkspaceOperationRequest,
  WorkspaceUpdate,
} from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

function headers(): { 'Idempotency-Key': string } {
  return mutationHeaders(`ui_${crypto.randomUUID()}`) as { 'Idempotency-Key': string }
}

export async function loadWorkspaces(): Promise<Array<Workspace>> {
  const result = await listWorkspaces()
  if (result.error || !result.data) throw new Error('Workspaces are unavailable.')
  return result.data
}

export async function loadWorkspace(workspaceId: string): Promise<Workspace> {
  const result = await getWorkspace({ path: { workspaceId } })
  if (result.error || !result.data) throw new Error('The workspace is unavailable.')
  return result.data
}

export async function saveWorkspace(definition: WorkspaceDefinition): Promise<Workspace> {
  const result = await createWorkspace({ body: definition, headers: headers() })
  if (result.error || !result.data) throw new Error('The workspace could not be created.')
  return result.data
}

export async function replaceWorkspace(
  workspaceId: string,
  definition: WorkspaceUpdate,
): Promise<Workspace> {
  const result = await updateWorkspace({
    path: { workspaceId },
    body: definition,
    headers: headers(),
  })
  if (result.error || !result.data) throw new Error('The workspace could not be updated.')
  return result.data
}

export async function removeWorkspace(workspaceId: string): Promise<void> {
  const result = await deleteWorkspace({ path: { workspaceId }, headers: headers() })
  if (result.error) throw new Error('The workspace could not be removed.')
}

export async function runWorkspace(
  workspaceId: string,
  request: WorkspaceOperationRequest,
): Promise<Operation> {
  const result = await createWorkspaceOperation({
    path: { workspaceId },
    body: request,
    headers: headers(),
  })
  if (result.error || !result.data) throw new Error('The workspace operation could not be queued.')
  return result.data
}
