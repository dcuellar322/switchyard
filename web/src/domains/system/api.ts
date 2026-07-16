import { getSystem } from '../../api/generated/sdk.gen'
import type { SystemInfo } from '../../api/generated/types.gen'

export async function loadSystemInfo(): Promise<SystemInfo> {
  const result = await getSystem()
  if (result.error || !result.data) {
    throw new Error('The local Switchyard daemon did not return system status.')
  }
  return result.data
}
