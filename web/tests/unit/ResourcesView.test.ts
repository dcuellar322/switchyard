import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import type { ResourceMetricPoint, ResourceOverview, ResourceProjectSnapshot, StorageInventory } from '../../src/api/generated/types.gen'

const api = vi.hoisted(() => ({
  loadCleanupPreview: vi.fn(),
  loadMetricHistory: vi.fn(),
  loadResourceOverview: vi.fn(),
  loadStorageInventory: vi.fn(),
}))

vi.mock('../../src/domains/resources/api', () => api)

import ResourcesView from '../../src/domains/resources/views/ResourcesView.vue'

afterEach(cleanup)

const observedAt = new Date().toISOString()

function metric(projectId: string, serviceId = '', overrides: Partial<ResourceMetricPoint> = {}): ResourceMetricPoint {
  return {
    timestamp: observedAt,
    projectId,
    serviceId,
    resolutionSeconds: 0,
    sampleCount: 1,
    cpuPercent: 4,
    cpuMaxPercent: 4,
	cpuAvailable: true,
    memoryBytes: 67_108_864,
    memoryMaxBytes: 67_108_864,
    memoryLimit: 268_435_456,
	memoryAvailable: true,
    networkRxBytes: 0,
    networkTxBytes: 0,
    networkAvailable: false,
    diskReadBytes: 0,
    diskWriteBytes: 0,
    diskAvailable: false,
    processCount: 1,
    restartCount: 0,
    healthLatencyMs: 0,
    healthAvailable: false,
    storageClassification: 'unknown',
    partial: false,
    ...overrides,
  }
}

function project(projectId: string, name: string, partial = false): ResourceProjectSnapshot {
  return {
    projectId,
    name,
    driver: 'process',
    state: 'running',
    active: true,
    metric: metric(projectId, '', { partial }),
    services: [{ serviceId: 'web', metric: metric(projectId, 'web', { partial }) }],
    budget: { cpuPercent: 80, memoryBytes: 536_870_912, storageBytes: 0 },
    warnings: [],
  }
}

function overview(projects: Array<ResourceProjectSnapshot>): ResourceOverview {
  return {
    observedAt,
    projects,
    storage: { bytes: 12_288, reclaimableBytes: 4_096, classification: 'shared', resourceCount: 2 },
    footprint: { databaseBytes: 8_192, databaseWalBytes: 0, databaseShmBytes: 0, logBytes: 4_096, logSegments: 1, metricRows: 12, classification: 'exclusive' },
    retention: { sampleIntervalSeconds: 10, rawSeconds: 3_600, minuteSeconds: 86_400, quarterHourSeconds: 2_592_000, maximumHistoryPoints: 1_000, logSeconds: 604_800, logBytes: 536_870_912 },
    warnings: [],
  }
}

const storage: StorageInventory = {
  connected: true,
  observedAt,
  summary: { bytes: 12_288, reclaimableBytes: 4_096, classification: 'shared', resourceCount: 1 },
  projects: [],
  resources: [{ kind: 'volume', id: 'beta-data', name: 'beta-data', projectIds: ['beta'], bytes: 12_288, reclaimable: true, classification: 'exclusive', reason: 'Canonical project label.' }],
  warnings: [],
}

beforeEach(() => {
  vi.clearAllMocks()
  api.loadStorageInventory.mockResolvedValue(storage)
  api.loadMetricHistory.mockImplementation(async (projectId: string, serviceId: string) => ({ projectId, serviceId, resolutionSeconds: 0, from: observedAt, to: observedAt, points: [metric(projectId, serviceId)] }))
})

async function renderView(path = '/resources') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/resources', name: 'resources', component: ResourcesView },
      { path: '/projects', name: 'projects', component: { template: '<div />' } },
    ],
  })
  await router.push(path)
  await router.isReady()
  const result = render(ResourcesView, {
    global: {
      plugins: [
        router,
        [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }) }],
      ],
    },
  })
  return { ...result, router }
}

test('shows loading, daemon failure, and trusted-project empty states', async () => {
  api.loadResourceOverview.mockReturnValueOnce(new Promise(() => {}))
  const loading = await renderView()
  expect(loading.container.querySelector('.loading')).toHaveAttribute('aria-live', 'polite')
  loading.unmount()

  api.loadResourceOverview.mockRejectedValueOnce(new Error('offline'))
  const failed = await renderView()
  expect(await screen.findByRole('alert')).toHaveTextContent('Resource intelligence unavailable')
  failed.unmount()

  api.loadResourceOverview.mockResolvedValueOnce(overview([]))
  await renderView()
  expect(await screen.findByText('No trusted projects')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Open project onboarding' })).toHaveAttribute('href', '/projects')
})

test('preserves project context across consumers, history, storage, and the URL', async () => {
  api.loadResourceOverview.mockResolvedValue(overview([project('alpha', 'Alpha App'), project('beta', 'Beta App', true)]))
  const { router } = await renderView('/resources?project=beta')

  expect(await screen.findByRole('heading', { name: 'Project and service consumption' })).toBeInTheDocument()
  await waitFor(() => expect(api.loadMetricHistory).toHaveBeenCalledWith('beta', '', '1h'))
  const projectSelects = screen.getAllByLabelText('Project') as Array<HTMLSelectElement>
  expect(projectSelects).toHaveLength(2)
  expect(projectSelects.every((select) => select.value === 'beta')).toBe(true)
  expect(screen.getAllByText('Partial').length).toBeGreaterThan(0)
  expect(screen.getByText('beta-data')).toBeInTheDocument()

  await fireEvent.click(screen.getByRole('button', { name: /Alpha App/ }))
  await waitFor(() => expect(router.currentRoute.value.query.project).toBe('alpha'))
  await waitFor(() => expect(api.loadMetricHistory).toHaveBeenCalledWith('alpha', '', '1h'))
  expect((screen.getAllByLabelText('Project') as Array<HTMLSelectElement>).every((select) => select.value === 'alpha')).toBe(true)
  expect(screen.queryByText('beta-data')).not.toBeInTheDocument()
})

test('keeps partial observations usable while storage refresh fails independently', async () => {
  api.loadResourceOverview.mockResolvedValue({ ...overview([project('alpha', 'Alpha App', true)]), warnings: ['One process exited during collection.'] })
  api.loadStorageInventory.mockRejectedValueOnce(new Error('docker disconnected'))
  await renderView()

  expect(await screen.findByText(/Partial observation: One process exited/)).toHaveAttribute('role', 'status')
  expect(screen.getAllByText('Partial').length).toBeGreaterThan(0)
  expect(await screen.findByText('Storage inventory could not be read.')).toHaveAttribute('role', 'alert')
  expect(screen.getByRole('heading', { name: 'Time-series evidence' })).toBeInTheDocument()
})
