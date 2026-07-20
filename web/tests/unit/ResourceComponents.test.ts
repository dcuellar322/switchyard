import { cleanup, fireEvent, render, screen } from '@testing-library/vue'
import { afterEach, expect, test, vi } from 'vitest'

import type {
  CleanupPreview,
  MetricHistory,
  ResourceMetricPoint,
  ResourceProjectSnapshot,
  StorageInventory,
} from '../../src/api/generated/types.gen'
import ResourceConsumersTable from '../../src/domains/resources/components/ResourceConsumersTable.vue'
import ResourceHistoryPanel from '../../src/domains/resources/components/ResourceHistoryPanel.vue'
import StorageIntelligence from '../../src/domains/resources/components/StorageIntelligence.vue'

afterEach(cleanup)

const observedAt = '2026-07-16T12:00:00Z'

function metric(overrides: Partial<ResourceMetricPoint> = {}): ResourceMetricPoint {
  return {
    timestamp: observedAt,
    projectId: 'alpha',
    serviceId: '',
    resolutionSeconds: 0,
    sampleCount: 1,
    cpuPercent: 12.5,
    cpuMaxPercent: 12.5,
    cpuAvailable: true,
    memoryBytes: 134_217_728,
    memoryMaxBytes: 134_217_728,
    memoryLimit: 536_870_912,
    memoryAvailable: true,
    networkRxBytes: 2_048,
    networkTxBytes: 1_024,
    networkAvailable: true,
    diskReadBytes: 4_096,
    diskWriteBytes: 8_192,
    diskAvailable: true,
    processCount: 2,
    restartCount: 0,
    healthLatencyMs: 10,
    healthAvailable: true,
    storageClassification: 'unknown',
    partial: false,
    ...overrides,
  }
}

const projects: Array<ResourceProjectSnapshot> = [
  {
    projectId: 'alpha',
    name: 'Alpha App',
    driver: 'compose',
    state: 'running',
    active: true,
    metric: metric(),
    services: [{ serviceId: 'api', metric: metric({ serviceId: 'api' }) }],
    budget: { cpuPercent: 100, memoryBytes: 536_870_912, storageBytes: 0 },
    warnings: [],
  },
]

const inventory: StorageInventory = {
  connected: true,
  observedAt,
  summary: { bytes: 30_720, reclaimableBytes: 20_480, classification: 'shared', resourceCount: 3 },
  projects: [
    {
      projectId: 'alpha',
      summary: {
        bytes: 10_240,
        reclaimableBytes: 10_240,
        classification: 'exclusive',
        resourceCount: 1,
      },
      unknownSizes: 0,
      sharedResources: 1,
    },
  ],
  resources: [
    {
      kind: 'volume',
      id: 'alpha-data',
      name: 'alpha-data',
      projectIds: ['alpha'],
      bytes: 10_240,
      reclaimable: true,
      classification: 'exclusive',
      reason: 'Canonical Compose project label.',
    },
    {
      kind: 'image',
      id: 'sha256:shared',
      name: 'shared:latest',
      projectIds: ['alpha', 'beta'],
      bytes: 20_480,
      reclaimable: false,
      classification: 'shared',
      reason: 'Referenced by more than one project.',
    },
    {
      kind: 'build_cache',
      id: 'cache-unknown',
      name: '',
      projectIds: [],
      reclaimable: true,
      classification: 'unknown',
      reason: 'Docker does not provide project ownership.',
    },
  ],
  warnings: [],
}

test('filters storage by durable project context and exposes only a non-executable cleanup preview', async () => {
  const onSelectProject = vi.fn()
  const onPreview = vi.fn()
  const view = render(StorageIntelligence, {
    props: {
      inventory,
      projects,
      projectId: 'alpha',
      loading: false,
      pending: false,
      inventoryError: false,
      previewError: '',
      onSelectProject,
      onPreview,
    },
  })

  expect(screen.getByText('Inspection only · no delete capability')).toBeInTheDocument()
  expect(screen.getByText('alpha-data')).toBeInTheDocument()
  expect(screen.getByText('shared:latest')).toBeInTheDocument()
  expect(screen.queryByText('cache-unknown')).not.toBeInTheDocument()

  await fireEvent.update(screen.getByLabelText('Project'), '')
  expect(onSelectProject).toHaveBeenCalledWith('')
  expect(screen.getByText('cache-unknown')).toBeInTheDocument()
  await fireEvent.update(screen.getByLabelText('Kind'), 'volume')
  expect(screen.getByText('alpha-data')).toBeInTheDocument()
  expect(screen.queryByText('shared:latest')).not.toBeInTheDocument()

  await fireEvent.click(screen.getByRole('button', { name: 'Preview reclaimable' }))
  expect(onPreview).toHaveBeenCalledWith('')

  const preview: CleanupPreview = {
    projectId: '',
    risk: 'destructive',
    executable: false,
    estimatedBytes: 10_240,
    unknownSizes: 1,
    resources: [inventory.resources[0]!, inventory.resources[2]!],
    warnings: ['Review every candidate before using Docker tooling.'],
    observedAt,
  }
  await view.rerender({ preview })
  expect(screen.getByRole('heading', { name: '2 reclaimable candidates' })).toBeInTheDocument()
  expect(screen.getByText('This preview cannot execute cleanup.')).toBeInTheDocument()
  expect(screen.getByText(/unknown and excluded/)).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /delete|prune|clean/i })).not.toBeInTheDocument()
})

test('keeps disconnected and failed storage states honest and actionable', async () => {
  const onRetry = vi.fn()
  const view = render(StorageIntelligence, {
    props: {
      inventory: { ...inventory, connected: false },
      projects,
      projectId: 'alpha',
      loading: false,
      pending: false,
      inventoryError: false,
      previewError: '',
      onRetry,
    },
  })

  expect(screen.getByRole('status')).toHaveTextContent('Docker is disconnected')
  expect(screen.getByRole('button', { name: 'Preview reclaimable' })).toBeDisabled()
  await view.rerender({ inventoryError: true, previewError: 'Cleanup preview failed.' })
  expect(screen.getByText(/Storage inventory could not be read/)).toHaveAttribute('role', 'alert')
  await fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
  expect(onRetry).toHaveBeenCalledOnce()
  expect(screen.getByText('Cleanup preview failed.')).toHaveAttribute('role', 'alert')
})

test('renders selectable history with a chart and accessible retained samples', async () => {
  const onSelectService = vi.fn()
  const onSelectRange = vi.fn()
  const history: MetricHistory = {
    projectId: 'alpha',
    serviceId: '',
    resolutionSeconds: 60,
    from: '2026-07-16T11:00:00Z',
    to: observedAt,
    points: [
      metric({ timestamp: '2026-07-16T11:59:00Z' }),
      metric({
        timestamp: observedAt,
        cpuPercent: 25,
        cpuMaxPercent: 30,
        memoryBytes: 268_435_456,
        memoryMaxBytes: 300_000_000,
        sampleCount: 6,
        partial: true,
      }),
    ],
  }
  const view = render(ResourceHistoryPanel, {
    props: {
      projects,
      projectId: 'alpha',
      serviceId: '',
      range: '1h',
      history,
      pending: false,
      error: false,
      onSelectService,
      onSelectRange,
    },
  })

  expect(screen.getByRole('img', { name: 'CPU history for Alpha App' })).toBeInTheDocument()
  expect(screen.getByText('Partial')).toBeInTheDocument()
  await fireEvent.update(screen.getByLabelText('Metric'), 'memory')
  expect(screen.getByRole('img', { name: 'Memory history for Alpha App' })).toBeInTheDocument()
  await fireEvent.update(screen.getByLabelText('Service'), 'api')
  await fireEvent.update(screen.getByLabelText('Range'), '24h')
  expect(onSelectService).toHaveBeenCalledWith('api')
  expect(onSelectRange).toHaveBeenCalledWith('24h')

  await view.rerender({ history: { ...history, points: [] } })
  expect(screen.getByText('No retained samples exist in this range yet.')).toBeInTheDocument()
  await view.rerender({ error: true })
  expect(screen.getByRole('alert')).toHaveTextContent('History could not be read')
})

test('renders unavailable resource evidence as gaps rather than zero consumption', async () => {
  const unavailable = metric({
    cpuPercent: 0,
    cpuAvailable: false,
    memoryBytes: 0,
    memoryAvailable: false,
    partial: true,
  })
  render(ResourceConsumersTable, {
    props: {
      projects: [
        {
          ...projects[0]!,
          metric: unavailable,
          services: [{ serviceId: 'api', metric: { ...unavailable, serviceId: 'api' } }],
        },
      ],
      selectedProject: 'alpha',
      selectedService: '',
    },
  })
  expect(screen.getAllByText('—')).toHaveLength(4)
  cleanup()

  render(ResourceHistoryPanel, {
    props: {
      projects,
      projectId: 'alpha',
      serviceId: '',
      range: '1h',
      history: {
        projectId: 'alpha',
        serviceId: '',
        resolutionSeconds: 0,
        from: observedAt,
        to: observedAt,
        points: [unavailable],
      },
      pending: false,
      error: false,
    },
  })
  expect(screen.getByText(/1 unavailable sample remains a gap/)).toBeInTheDocument()
  expect(screen.getByText('Unavailable')).toBeInTheDocument()
})
