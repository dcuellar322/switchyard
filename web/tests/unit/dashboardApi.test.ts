import { beforeEach, expect, test, vi } from 'vitest'

import type { Project } from '../../src/api/generated/types.gen'

const projects = vi.hoisted(() => ({
  loadProjectActions: vi.fn(),
  loadProjectGit: vi.fn(),
  loadProjectMetrics: vi.fn(),
  loadProjectRuntime: vi.fn(),
}))
vi.mock('../../src/domains/projects/api', () => projects)

import { loadProjectSnapshot, loadProjectSnapshots } from '../../src/domains/dashboard/api'

const project: Project = {
  id: 'project-1',
  slug: 'example',
  displayName: 'Example',
  trustState: 'trusted',
  primaryLocation: '/workspace/example',
  tags: [],
  manifestRevision: 1,
  createdAt: '2026-07-16T12:00:00Z',
  updatedAt: '2026-07-16T12:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  projects.loadProjectRuntime.mockResolvedValue({ state: 'running' })
  projects.loadProjectGit.mockResolvedValue({ branch: 'main' })
  projects.loadProjectMetrics.mockResolvedValue([{ memoryBytes: 1024 }])
  projects.loadProjectActions.mockResolvedValue({ actions: [] })
})

test('assembles available project observations without hiding partial failures', async () => {
  projects.loadProjectGit.mockRejectedValueOnce(new Error('not a repository'))
  projects.loadProjectActions.mockRejectedValueOnce(new Error('manifest unavailable'))
  const snapshot = await loadProjectSnapshot(project)
  expect(snapshot.project).toBe(project)
  expect(snapshot.runtime).toEqual({ state: 'running' })
  expect(snapshot.git).toBeUndefined()
  expect(snapshot.actions).toBeUndefined()
  expect(snapshot.warnings).toEqual(['Git observation unavailable', 'Quick actions unavailable'])
})

test('loads project observations in bounded batches while preserving catalog order', async () => {
  const catalog = Array.from({ length: 5 }, (_, index) => ({
    ...project,
    id: `project-${index}`,
    slug: `project-${index}`,
    displayName: `Project ${index}`,
  }))
  const snapshots = await loadProjectSnapshots(catalog, 2)
  expect(snapshots.map((snapshot) => snapshot.project.id)).toEqual(catalog.map((item) => item.id))
  expect(projects.loadProjectRuntime).toHaveBeenCalledTimes(5)
})
