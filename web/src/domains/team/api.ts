import {
  getEffectiveTeamPolicy,
  installTeamBundle,
  listCuratedPlugins,
  listTeamBundles,
  listTeamPublishers,
  trustTeamPublisher,
} from '../../api/generated/sdk.gen'
import type {
  CuratedPlugin,
  EffectiveTeamPolicy,
  TeamBundle,
  TeamPublisher,
} from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

function headers(): Record<string, string> {
  return mutationHeaders(`ui_${crypto.randomUUID()}`)
}

export async function loadTeamPublishers(): Promise<Array<TeamPublisher>> {
  const result = await listTeamPublishers()
  if (result.error || !result.data) throw new Error('Trusted team publishers are unavailable.')
  return result.data
}

export async function trustPublisher(name: string, publicKey: string): Promise<TeamPublisher> {
  const result = await trustTeamPublisher({
    body: { name, publicKey, confirmRisk: true },
    headers: headers(),
  })
  if (result.error || !result.data) throw new Error('The exact publisher key could not be trusted.')
  return result.data
}

export async function loadTeamBundles(): Promise<Array<TeamBundle>> {
  const result = await listTeamBundles()
  if (result.error || !result.data) throw new Error('Signed team bundles are unavailable.')
  return result.data
}

export async function installBundle(bundle: TeamBundle): Promise<TeamBundle> {
  const result = await installTeamBundle({
    body: { bundle, confirmRisk: true },
    headers: headers(),
  })
  if (result.error || !result.data)
    throw new Error('The signed bundle could not be verified and installed.')
  return result.data
}

export async function loadEffectivePolicy(): Promise<EffectiveTeamPolicy> {
  const result = await getEffectiveTeamPolicy()
  if (result.error || !result.data) throw new Error('Effective team policy is unavailable.')
  return result.data
}

export async function loadCuratedPlugins(): Promise<Array<CuratedPlugin>> {
  const result = await listCuratedPlugins()
  if (result.error || !result.data) throw new Error('The signed plugin registry is unavailable.')
  return result.data
}
