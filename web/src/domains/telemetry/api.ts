import {
  getTelemetryStatus,
  sendTelemetryNow,
  updateTelemetrySettings,
} from '../../api/generated/sdk.gen'
import type { TelemetryStatus } from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

function headers(): Record<string, string> {
  return mutationHeaders(`ui_${crypto.randomUUID()}`)
}

export async function loadTelemetryStatus(): Promise<TelemetryStatus> {
  const result = await getTelemetryStatus()
  if (result.error || !result.data) throw new Error('Anonymous metrics status is unavailable.')
  return result.data
}

export async function enableTelemetry(endpoint: string): Promise<TelemetryStatus> {
  const result = await updateTelemetrySettings({
    body: { enabled: true, endpoint, confirmRisk: true },
    headers: headers(),
  })
  if (result.error || !result.data) throw new Error('Anonymous metrics could not be enabled.')
  return result.data
}

export async function disableTelemetry(): Promise<TelemetryStatus> {
  const result = await updateTelemetrySettings({
    body: { enabled: false, confirmRisk: false },
    headers: headers(),
  })
  if (result.error || !result.data) throw new Error('Anonymous metrics could not be disabled.')
  return result.data
}

export async function sendTelemetry(): Promise<TelemetryStatus> {
  const result = await sendTelemetryNow({ headers: headers() })
  if (result.error || !result.data) throw new Error('Anonymous metrics could not be sent.')
  return result.data
}
