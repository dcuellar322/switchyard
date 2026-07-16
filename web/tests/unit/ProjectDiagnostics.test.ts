import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { render, screen } from '@testing-library/vue'
import { ref } from 'vue'
import { expect, test, vi } from 'vitest'

vi.mock('../../src/domains/projects/api', () => ({
  loadProjectRuntime: vi.fn().mockResolvedValue({
    projectId: 'project-1',
    driver: 'process',
    projectIdentity: 'project-one',
    state: 'degraded',
    origin: 'switchyard',
    observedAt: '2026-07-16T12:00:00Z',
    services: [{ id: 'api', runtimeName: 'api', state: 'running', health: '', ports: [], observedAt: '2026-07-16T12:00:00Z' }],
  }),
  loadProjectHealth: vi.fn().mockResolvedValue({
    projectId: 'project-1',
    status: 'unhealthy',
    observerState: 'disconnected',
    observedAt: '2026-07-16T12:00:00Z',
    results: [{ projectId: 'project-1', serviceId: 'api', checkId: 'ready', type: 'http', status: 'unhealthy', severity: 'critical', required: true, latencyMs: 12, message: 'HTTP endpoint is unavailable', observedAt: '2026-07-16T12:00:00Z' }],
  }),
  loadProjectLogs: vi.fn().mockResolvedValue([{ sequence: 7, timestamp: '2026-07-16T12:00:00Z', projectId: 'project-1', serviceId: 'api', runId: 'run-1', source: 'process', stream: 'stdout', level: 'info', message: 'token=[REDACTED]', redacted: true, attributes: {} }]),
  loadProjectGit: vi.fn().mockResolvedValue({
    projectId: 'project-1', repository: true, branch: 'main', detached: false, ahead: 2, behind: 0,
    changes: { staged: 1, modified: 1, untracked: 0, conflicted: 0 }, stashes: 1, remotes: [],
    worktrees: [{ path: '/repo/project-one', head: 'abc', branch: 'main', detached: false, bare: false, locked: false }],
    observedAt: '2026-07-16T12:00:00Z', lastCommit: { hash: 'abcdef', shortHash: 'abcdef', author: 'Test', subject: 'phase eight', committedAt: '2026-07-16T12:00:00Z' },
  }),
  loadProjectActions: vi.fn().mockResolvedValue({ projectId: 'project-1', projectName: 'Project One', actions: [
    { id: 'terminal', name: 'Open terminal', type: 'terminal.open', command: [], workingDirectory: '.', shell: false, captureOutput: false, risk: 'interactive', timeoutSeconds: 0 },
  ] }),
  runProjectAction: vi.fn().mockResolvedValue({ id: 'operation-action', projectId: 'project-1', kind: 'action.run', state: 'queued', cancellationRequested: false, requestedAt: '2026-07-16T12:00:00Z', updatedAt: '2026-07-16T12:00:00Z' }),
}))

vi.mock('../../src/domains/projects/composables/useProjectLogStream', () => ({
  useProjectLogStream: () => ({ state: ref('disconnected'), lastSequence: ref(7) }),
}))

import ProjectDiagnostics from '../../src/domains/projects/components/ProjectDiagnostics.vue'
import { loadProjectHealth, loadProjectRuntime } from '../../src/domains/projects/api'

test('renders degraded, disconnected, health, and redacted log states honestly', async () => {
  render(ProjectDiagnostics, {
    props: { project: { id: 'project-1', slug: 'project-one', displayName: 'Project One', description: '', trustState: 'trusted', primaryLocation: '/repo/project-one', tags: [], manifestRevision: 1, createdAt: '2026-07-16T12:00:00Z', updatedAt: '2026-07-16T12:00:00Z' } },
    global: { plugins: [[VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })

  expect(await screen.findByText('Runtime observer disconnected. Last-known health is not treated as current.')).toBeInTheDocument()
  expect(screen.getByText('degraded')).toBeInTheDocument()
  expect(screen.getByText('HTTP endpoint is unavailable · 12 ms')).toBeInTheDocument()
  expect(screen.getByText('token=[REDACTED]')).toBeInTheDocument()
  expect(screen.getByText('redacted')).toBeInTheDocument()
  expect(screen.getByText('disconnected')).toBeInTheDocument()
  expect(screen.getByText('main')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Open terminal/ })).toBeInTheDocument()
})

test('renders empty diagnostics when a legacy response contains null collections', async () => {
  vi.mocked(loadProjectRuntime).mockResolvedValueOnce({
    projectId: 'project-legacy', driver: 'process', projectIdentity: 'legacy', state: 'stopped', origin: 'switchyard',
    observedAt: '2026-07-16T12:00:00Z', services: null,
  } as never)
  vi.mocked(loadProjectHealth).mockResolvedValueOnce({
    projectId: 'project-legacy', status: 'unknown', observerState: 'connected', observedAt: '2026-07-16T12:00:00Z', results: null,
  } as never)

  render(ProjectDiagnostics, {
    props: { project: { id: 'project-legacy', slug: 'legacy', displayName: 'Legacy Project', description: '', trustState: 'trusted', primaryLocation: '/repo/legacy', tags: [], manifestRevision: 1, createdAt: '2026-07-16T12:00:00Z', updatedAt: '2026-07-16T12:00:00Z' } },
    global: { plugins: [[VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })

  expect(await screen.findByText('No service observations are available.')).toBeInTheDocument()
  expect(screen.getByText('No health checks are declared for this project.')).toBeInTheDocument()
})
