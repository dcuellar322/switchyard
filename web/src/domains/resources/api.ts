import {
  getCleanupPreview,
  getMetricHistory,
  getResourceOverview,
  getStorageInventory,
} from '../../api/generated/sdk.gen'
import type {
  CleanupPreview,
  MetricHistory,
  ResourceOverview,
  StorageInventory,
} from '../../api/generated/types.gen'

export async function loadResourceOverview(): Promise<ResourceOverview> {
  const result = await getResourceOverview()
  if (result.error || !result.data) throw new Error('Resource intelligence is unavailable.')
  return result.data
}

export async function loadStorageInventory(): Promise<StorageInventory> {
  const result = await getStorageInventory()
  if (result.error || !result.data) throw new Error('Storage inventory is unavailable.')
  return result.data
}

export async function loadCleanupPreview(projectId = ''): Promise<CleanupPreview> {
  const result = await getCleanupPreview({ query: projectId ? { projectId } : {} })
  if (result.error || !result.data) throw new Error('Cleanup preview is unavailable.')
  return result.data
}

export async function loadMetricHistory(
  projectId: string,
  service: string,
  range: '1h' | '24h' | '7d',
): Promise<MetricHistory> {
  const to = new Date()
  const duration = range === '1h' ? 3_600_000 : range === '24h' ? 86_400_000 : 7 * 86_400_000
  const result = await getMetricHistory({
    path: { projectId },
    query: {
      service: service || undefined,
      from: new Date(to.getTime() - duration).toISOString(),
      to: to.toISOString(),
      resolution: 'auto',
      maxPoints: 1_000,
    },
  })
  if (result.error || !result.data) throw new Error('Metric history is unavailable.')
  return result.data
}
