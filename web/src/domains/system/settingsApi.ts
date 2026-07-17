import { getDaemonSettings, updateDaemonSettings } from '../../api/generated/sdk.gen'
import type { DaemonSettings, DaemonSettingsStatus, ProblemDetails } from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

export class SettingsAPIError extends Error {
  constructor(message: string, readonly code = '') {
    super(message)
    this.name = 'SettingsAPIError'
  }
}

function problemMessage(problem: ProblemDetails | undefined, fallback: string): SettingsAPIError {
  return new SettingsAPIError(problem?.detail || fallback, problem?.code)
}

export async function loadDaemonSettings(): Promise<DaemonSettingsStatus> {
  const result = await getDaemonSettings()
  if (result.error || !result.data) {
    throw problemMessage(result.error, 'Durable daemon settings are unavailable.')
  }
  return result.data
}

export async function saveDaemonSettings(settings: DaemonSettings): Promise<DaemonSettingsStatus> {
  const result = await updateDaemonSettings({
    body: { expectedRevision: settings.revision, settings },
    headers: mutationHeaders(`ui_${crypto.randomUUID()}`) as { 'Idempotency-Key': string },
  })
  if (result.error || !result.data) {
    throw problemMessage(result.error, 'Durable daemon settings could not be saved.')
  }
  return result.data
}
