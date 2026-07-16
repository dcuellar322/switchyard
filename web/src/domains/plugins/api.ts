import {
  checkPluginHealth,
  disablePlugin,
  enablePlugin,
  listPluginLogs,
  listPlugins,
  refreshPlugins,
  trustPlugin,
} from '../../api/generated/sdk.gen'
import type { PluginEnableRequest, PluginLogEntry, PluginRegistration } from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

function headers(): Record<string, string> {
  return mutationHeaders(`ui_${crypto.randomUUID()}`)
}

export async function loadPlugins(): Promise<Array<PluginRegistration>> {
  const result = await listPlugins()
  if (result.error || !result.data) throw new Error('Plugin registrations are unavailable.')
  return result.data
}

export async function discoverPlugins(): Promise<Array<PluginRegistration>> {
  const result = await refreshPlugins({ headers: headers() })
  if (result.error || !result.data) throw new Error('Plugin discovery could not be refreshed.')
  return result.data
}

export async function approvePlugin(pluginId: string, fingerprint: string): Promise<PluginRegistration> {
  const result = await trustPlugin({ path: { pluginId }, body: { fingerprint }, headers: headers() })
  if (result.error || !result.data) throw new Error('The exact plugin fingerprint could not be trusted.')
  return result.data
}

export async function activatePlugin(pluginId: string, grantedScopes: PluginEnableRequest['grantedScopes']): Promise<PluginRegistration> {
  const result = await enablePlugin({ path: { pluginId }, body: { grantedScopes }, headers: headers() })
  if (result.error || !result.data) throw new Error('The plugin could not be enabled with these scopes.')
  return result.data
}

export async function deactivatePlugin(pluginId: string): Promise<PluginRegistration> {
  const result = await disablePlugin({ path: { pluginId }, headers: headers() })
  if (result.error || !result.data) throw new Error('The plugin could not be disabled.')
  return result.data
}

export async function probePlugin(pluginId: string): Promise<PluginRegistration> {
  const result = await checkPluginHealth({ path: { pluginId }, headers: headers() })
  if (result.error || !result.data) throw new Error('The supervised plugin health check failed.')
  return result.data
}

export async function loadPluginLogs(pluginId: string): Promise<Array<PluginLogEntry>> {
  const result = await listPluginLogs({ path: { pluginId }, query: { limit: 100 } })
  if (result.error || !result.data) throw new Error('Plugin supervision logs are unavailable.')
  return result.data
}
