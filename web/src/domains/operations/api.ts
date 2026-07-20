import { cancelOperation, listOperations } from '../../api/generated/sdk.gen'
import type { Operation } from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

export async function loadOperations(): Promise<Array<Operation>> {
  const result = await listOperations({ query: { limit: 100 } })
  if (result.error || !result.data) throw new Error('Recent operations are unavailable.')
  return result.data
}

export async function requestOperationCancellation(operationId: string): Promise<Operation> {
  const result = await cancelOperation({
    path: { operationId },
    headers: mutationHeaders(`ui_${crypto.randomUUID()}`) as {
      'Idempotency-Key': string
    },
  })
  if (result.error || !result.data) throw new Error('The operation could not be cancelled.')
  return result.data
}
