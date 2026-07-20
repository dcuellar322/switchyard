import {
  createMachine,
  createMachineOperation,
  deleteMachine,
  getMachineSnapshot,
  listMachines,
  probeMachine,
  updateMachineAccess,
} from '../../api/generated/sdk.gen'
import type {
  FleetSnapshot,
  Machine,
  MachineAccessRequest,
  MachineRegistrationRequest,
  RemoteOperationReceipt,
  RemoteOperationRequest,
} from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

function headers(): Record<string, string> {
  return mutationHeaders(`ui_${crypto.randomUUID()}`)
}

export async function loadMachines(): Promise<Array<Machine>> {
  const result = await listMachines()
  if (result.error || !result.data) throw new Error('Remote machine inventory is unavailable.')
  return result.data
}

export async function registerMachine(request: MachineRegistrationRequest): Promise<Machine> {
  const result = await createMachine({ body: request, headers: headers() })
  if (result.error || !result.data)
    throw new Error('The authenticated remote machine could not be registered.')
  return result.data
}

export async function refreshMachine(machineId: string): Promise<Machine> {
  const result = await probeMachine({ path: { machineId }, headers: headers() })
  if (result.error || !result.data)
    throw new Error('The remote machine identity could not be refreshed.')
  return result.data
}

export async function saveMachineAccess(
  machineId: string,
  request: MachineAccessRequest,
): Promise<Machine> {
  const result = await updateMachineAccess({
    path: { machineId },
    body: request,
    headers: headers(),
  })
  if (result.error || !result.data)
    throw new Error('The reviewed remote access could not be saved.')
  return result.data
}

export async function removeMachine(machineId: string): Promise<void> {
  const result = await deleteMachine({
    path: { machineId },
    query: { confirmRisk: true },
    headers: headers(),
  })
  if (result.error) throw new Error('The local remote-machine registration could not be removed.')
}

export async function loadMachineSnapshot(machineId: string): Promise<FleetSnapshot> {
  const result = await getMachineSnapshot({ path: { machineId } })
  if (result.error || !result.data) throw new Error('The bounded remote inventory is unavailable.')
  return result.data
}

export async function runMachineOperation(
  machineId: string,
  request: RemoteOperationRequest,
): Promise<RemoteOperationReceipt> {
  const result = await createMachineOperation({
    path: { machineId },
    body: request,
    headers: headers(),
  })
  if (result.error || !result.data)
    throw new Error('The remote lifecycle operation was not accepted.')
  return result.data
}
