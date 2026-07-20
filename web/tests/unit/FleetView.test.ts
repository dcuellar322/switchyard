import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { cleanup, render, screen } from '@testing-library/vue'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

const api = vi.hoisted(() => ({
  loadMachineSnapshot: vi.fn(),
  loadMachines: vi.fn(),
  refreshMachine: vi.fn(),
  registerMachine: vi.fn(),
  removeMachine: vi.fn(),
  runMachineOperation: vi.fn(),
  saveMachineAccess: vi.fn(),
}))
vi.mock('../../src/domains/fleet/api', () => api)
import FleetView from '../../src/domains/fleet/views/FleetView.vue'

const machine = {
  id: 'machine-1',
  name: 'Build box',
  endpoint: 'https://127.0.0.1:19618',
  certificateFingerprint: 'a'.repeat(64),
  credentialConfigured: true,
  enabled: true,
  capabilities: ['inventory.read', 'project.operate'],
  grantedCapabilities: ['inventory.read'],
  state: 'online',
  peerId: 'peer-1',
  peerVersion: '1.0.0',
  os: 'linux',
  architecture: 'amd64',
  createdAt: '2026-07-16T12:00:00Z',
  updatedAt: '2026-07-16T12:00:00Z',
} as const
const snapshot = {
  identity: {
    protocolVersion: 'switchyard.remote/v1',
    machineId: 'peer-1',
    name: 'Build box',
    version: '1.0.0',
    os: 'linux',
    architecture: 'amd64',
    capabilities: ['inventory.read', 'project.operate'],
  },
  projects: [
    {
      id: 'project-1',
      slug: 'example',
      displayName: 'Example API',
      runtime: 'compose',
      state: 'running',
      health: 'healthy',
      degraded: false,
    },
  ],
  environments: [],
  observedAt: '2026-07-16T12:00:00Z',
} as const

afterEach(cleanup)
beforeEach(() => {
  vi.clearAllMocks()
  api.loadMachines.mockResolvedValue([machine])
  api.loadMachineSnapshot.mockResolvedValue(snapshot)
})

function renderView(readOnly = false) {
  return render(FleetView, {
    props: { readOnly },
    global: {
      plugins: [
        [
          VueQueryPlugin,
          {
            queryClient: new QueryClient({
              defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
            }),
          },
        ],
      ],
      stubs: { RouterLink: { template: '<a><slot /></a>' } },
    },
  })
}

test('renders authenticated machine inventory and explicit local-first safety', async () => {
  renderView()
  expect(await screen.findByRole('heading', { name: 'Build box' })).toBeInTheDocument()
  expect(await screen.findByText('Example API')).toBeInTheDocument()
  expect(screen.getByText('Local-only remains the default.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Add remote machine' })).toBeInTheDocument()
})

test('companion is read-only and omits trust and mutation controls', async () => {
  renderView(true)
  expect(await screen.findByRole('heading', { name: 'Switchyard companion' })).toBeInTheDocument()
  expect(await screen.findByText('Example API')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Add remote machine' })).not.toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: 'Controller grants' })).not.toBeInTheDocument()
  expect(
    screen.queryByRole('heading', { name: 'Remote lifecycle operation' }),
  ).not.toBeInTheDocument()
})
