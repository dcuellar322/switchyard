import {
  acceptManifestProposal,
  createManifestProposal,
  listProjects,
  validateManifestProposal,
} from '../../api/generated/sdk.gen'
import type { AcceptedManifestProposal, ManifestProposal, Project } from '../../api/generated/types.gen'
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
