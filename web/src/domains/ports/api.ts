import { createPortSuggestion, getPortRegistry } from '../../api/generated/sdk.gen'
import type { PortRegistry, PortSuggestion } from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

export async function loadPortRegistry(): Promise<PortRegistry> {
  const result = await getPortRegistry()
  if (result.error || !result.data) throw new Error('The port registry is unavailable.')
  return result.data
}

export async function suggestPort(rangeStart = 15_000, rangeEnd = 19_999): Promise<PortSuggestion> {
  const result = await createPortSuggestion({
    body: { rangeStart, rangeEnd, protocol: 'tcp' },
    headers: mutationHeaders(`ui_${crypto.randomUUID()}`) as { 'Idempotency-Key': string },
  })
  if (result.error || !result.data) throw new Error('No port suggestion is currently available.')
  return result.data
}
