import { getHost, getSystem } from '../../api/generated/sdk.gen'
import type { HostObservation, SystemInfo } from '../../api/generated/types.gen'

export async function loadSystemInfo(): Promise<SystemInfo> {
  const result = await getSystem()
  if (result.error || !result.data) {
    throw new Error('The local Switchyard daemon did not return system status.')
  }
  return result.data
}

export async function loadHostObservation(): Promise<HostObservation> {
  const result = await getHost()
  if (result.error || !result.data) throw new Error('Host capacity is unavailable.')
  return result.data
}
