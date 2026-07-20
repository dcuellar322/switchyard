import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

const workspaceAPI = vi.hoisted(() => ({
  loadWorkspace: vi.fn(),
  loadWorkspaces: vi.fn(),
  runWorkspace: vi.fn(),
  saveWorkspace: vi.fn(),
}))
const projectAPI = vi.hoisted(() => ({ loadProjects: vi.fn() }))
const environmentAPI = vi.hoisted(() => ({ loadAllEnvironments: vi.fn() }))
const operations = vi.hoisted(() => ({ trackOperation: vi.fn() }))

vi.mock('../../src/domains/workspaces/api', () => workspaceAPI)
vi.mock('../../src/domains/projects/api', () => projectAPI)
vi.mock('../../src/domains/environments/api', () => environmentAPI)
vi.mock('../../src/domains/operations/store', () => operations)

import WorkspacesView from '../../src/domains/workspaces/views/WorkspacesView.vue'

const project = {
  id: 'project-api',
  slug: 'api',
  displayName: 'Example API',
  trustState: 'trusted',
  primaryLocation: '/workspace/api',
  tags: ['backend'],
  manifestRevision: 1,
  createdAt: '2026-07-16T12:00:00Z',
  updatedAt: '2026-07-16T12:00:00Z',
} as const

const workspace = {
  id: 'workspace-1',
  name: 'Release train',
  description: 'API and tools',
  policy: 'rollback',
  profile: 'full',
  members: [
    {
      projectId: project.id,
      role: 'application',
      order: 0,
      healthGate: true,
      healthTimeoutSeconds: 120,
      status: 'idle',
    },
  ],
  dependencies: [],
  recipes: [],
  profiles: [
    {
      id: 'full',
      name: 'Full workspace',
      projectIds: [project.id],
      maxParallel: 4,
      lowMemory: false,
    },
  ],
  revision: 1,
  createdAt: '2026-07-16T12:00:00Z',
  updatedAt: '2026-07-16T12:00:00Z',
} as const

afterEach(cleanup)
beforeEach(() => {
  vi.clearAllMocks()
  projectAPI.loadProjects.mockResolvedValue([project])
  environmentAPI.loadAllEnvironments.mockResolvedValue([])
  workspaceAPI.loadWorkspaces.mockResolvedValue([workspace])
  workspaceAPI.loadWorkspace.mockResolvedValue(workspace)
  workspaceAPI.saveWorkspace.mockResolvedValue(workspace)
  workspaceAPI.runWorkspace.mockResolvedValue({ id: 'operation-1', state: 'queued' })
})

async function renderView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/workspaces', component: WorkspacesView }],
  })
  await router.push('/workspaces')
  await router.isReady()
  return render(WorkspacesView, {
    global: {
      plugins: [
        router,
        [
          VueQueryPlugin,
          {
            queryClient: new QueryClient({
              defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
            }),
          },
        ],
      ],
    },
  })
}

test('renders a dependency graph and queues only valid lifecycle controls', async () => {
  await renderView()
  expect(await screen.findByRole('heading', { name: 'Release train' })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'Start order' })).toBeInTheDocument()
  const start = screen.getByRole('button', { name: /Start$/ })
  const stop = screen.getByRole('button', { name: /Stop all/ })
  expect(start).toBeEnabled()
  expect(stop).toBeDisabled()
  await fireEvent.click(start)
  await waitFor(() =>
    expect(workspaceAPI.runWorkspace).toHaveBeenCalledWith('workspace-1', {
      action: 'start',
      profileId: 'full',
      policy: 'rollback',
      runRecipes: false,
    }),
  )
  await waitFor(() => expect(operations.trackOperation).toHaveBeenCalled())
})

test('builds a workspace from trusted member choices and represents the empty state', async () => {
  workspaceAPI.loadWorkspaces.mockResolvedValueOnce([])
  await renderView()
  expect(
    await screen.findByRole('heading', { name: 'Create your first workspace' }),
  ).toBeInTheDocument()
  await fireEvent.click(screen.getByRole('button', { name: 'Build a workspace' }))
  expect(screen.getByRole('heading', { name: 'Coordinate related projects' })).toBeInTheDocument()
  await fireEvent.update(screen.getByLabelText('Name'), 'Product development')
  await fireEvent.click(await screen.findByRole('checkbox'))
  await fireEvent.click(screen.getByRole('button', { name: 'Create workspace' }))
  await waitFor(() => expect(workspaceAPI.saveWorkspace).toHaveBeenCalled())
  expect(workspaceAPI.saveWorkspace.mock.calls[0]?.[0]).toEqual(
    expect.objectContaining({
      name: 'Product development',
      members: [expect.objectContaining({ projectId: project.id })],
    }),
  )
})

test('enables stop after a successful start while member projects remain active', async () => {
  const active = {
    ...workspace,
    members: [{ ...workspace.members[0], status: 'running' }],
    lastRun: {
      id: 'run-1',
      workspaceId: workspace.id,
      kind: 'start',
      state: 'succeeded',
      policy: 'rollback',
      profileId: 'full',
      removeData: false,
      projects: [
        {
          projectId: project.id,
          role: 'application',
          status: 'running',
          message: 'Healthy',
          order: 0,
          startedAt: '2026-07-16T12:00:00Z',
        },
      ],
      startedAt: '2026-07-16T12:00:00Z',
    },
  }
  workspaceAPI.loadWorkspaces.mockResolvedValueOnce([active])
  workspaceAPI.loadWorkspace.mockResolvedValueOnce(active)
  await renderView()
  expect(await screen.findByRole('heading', { name: 'start · succeeded' })).toBeInTheDocument()
  const start = screen.getByRole('button', { name: /Start$/ })
  const stop = screen.getByRole('button', { name: /Stop all/ })
  expect(start).toBeDisabled()
  expect(stop).toBeEnabled()
  await fireEvent.click(stop)
  await waitFor(() =>
    expect(workspaceAPI.runWorkspace).toHaveBeenCalledWith(
      'workspace-1',
      expect.objectContaining({ action: 'stop' }),
    ),
  )
})
