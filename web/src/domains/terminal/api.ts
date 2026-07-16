import {
  createAgentSession,
  createTerminalSession,
  listAgentSessions,
  listTerminalSessions,
  terminateTerminalSession,
} from '../../api/generated/sdk.gen'
import type {
  AgentSessionCreate,
  TerminalSession,
  TerminalSessionCreate,
} from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

function headers(): { 'Idempotency-Key': string } {
  return mutationHeaders(`ui_${crypto.randomUUID()}`) as { 'Idempotency-Key': string }
}

export async function loadTerminalSessions(projectId: string): Promise<Array<TerminalSession>> {
  const result = await listTerminalSessions({ query: { projectId } })
  if (result.error || !result.data) throw new Error('Terminal sessions are unavailable.')
  return result.data
}

export async function startTerminalSession(request: TerminalSessionCreate): Promise<TerminalSession> {
  const result = await createTerminalSession({ body: request, headers: headers() })
  if (result.error || !result.data) throw new Error('The terminal session could not be started.')
  return result.data
}

export async function stopTerminalSession(terminalSessionId: string): Promise<TerminalSession> {
  const result = await terminateTerminalSession({ path: { terminalSessionId }, headers: headers() })
  if (result.error || !result.data) throw new Error('The terminal session could not be terminated.')
  return result.data
}

export async function loadAgentSessions(projectId: string): Promise<Array<TerminalSession>> {
  const result = await listAgentSessions({ query: { projectId } })
  if (result.error || !result.data) throw new Error('Agent sessions are unavailable.')
  return result.data
}

export async function startAgentSession(request: AgentSessionCreate): Promise<TerminalSession> {
  const result = await createAgentSession({ body: request, headers: headers() })
  if (result.error || !result.data) throw new Error('The agent session could not be started.')
  return result.data
}
