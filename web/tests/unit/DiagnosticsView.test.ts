import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

const api = vi.hoisted(() => ({
  acknowledgeNotification: vi.fn(), diagnoseProject: vi.fn(), evaluateRecipes: vi.fn(), loadLatestDiagnosis: vi.fn(),
  loadNotifications: vi.fn(), loadProviders: vi.fn(), loadRecipes: vi.fn(),
  reviewHypothesis: vi.fn(), runSuggestedAction: vi.fn(), saveRecipe: vi.fn(),
  setRecipeEnabled: vi.fn(),
}))
vi.mock('../../src/domains/diagnostics/api', () => api)
vi.mock('../../src/domains/projects/api', () => ({
  loadProjects: vi.fn().mockResolvedValue([{ id: 'alpha', displayName: 'Alpha API' }]),
}))

import DiagnosticsView from '../../src/domains/diagnostics/views/DiagnosticsView.vue'

const diagnosis = {
  id: 'diagnosis-1', version: 'switchyard.dev/diagnosis/v1alpha1', projectId: 'alpha',
  bundleSha256: 'a'.repeat(64), bundleBytes: 2480, generatedAt: '2026-07-16T15:20:00Z', deterministic: true,
  cleanupPreview: { estimatedBytes: 4096, candidates: 2, unknownSizes: 0, executable: false }, warnings: [],
  evidence: [{ id: 'runtime', kind: 'runtime', summary: 'Process exited three times', source: 'runtime', data: {}, untrusted: false, redacted: false, truncated: false, observedAt: '2026-07-16T15:20:00Z' }],
  hypotheses: [{ id: 'crash-loop', code: 'REPEATED_CRASH', title: 'Service is repeatedly crashing', summary: 'The API exited three times during the observation window.', severity: 'error', confidence: 0.99, source: 'deterministic', evidenceIds: ['runtime'], notifies: true, suggestedActions: [{ actionId: 'tests', name: 'Test suite', risk: 'mutating', reason: 'Existing accepted action' }] }],
} as const

afterEach(cleanup)
beforeEach(() => {
  vi.clearAllMocks()
  api.loadLatestDiagnosis.mockResolvedValue(diagnosis)
  api.loadNotifications.mockResolvedValue([])
  api.loadProviders.mockResolvedValue([])
  api.loadRecipes.mockResolvedValue([])
  api.acknowledgeNotification.mockResolvedValue({ id: 'notification-1' })
  api.reviewHypothesis.mockResolvedValue({ id: 'feedback-1' })
  api.runSuggestedAction.mockResolvedValue({ id: 'operation-1' })
})

function renderView() {
  return render(DiagnosticsView, {
    global: {
      plugins: [[VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }) }]],
    },
  })
}

test('shows deterministic evidence and only dispatches a validated suggestion', async () => {
  renderView()
  expect(await screen.findByText('Service is repeatedly crashing')).toBeInTheDocument()
  expect(screen.getByText('Deterministic')).toBeInTheDocument()
  expect(screen.getByText(/non-executable dry run/i)).toHaveTextContent('2 candidates')

  await fireEvent.click(screen.getByRole('button', { name: 'Test suite' }))
  await waitFor(() => expect(api.runSuggestedAction).toHaveBeenCalledWith('diagnosis-1', 'tests'))
  expect(await screen.findByRole('status')).toHaveTextContent('operation-1')
})

test('records false-positive review locally and creates recipes disabled', async () => {
  api.saveRecipe.mockResolvedValue({ id: 'recipe-1', enabled: false })
  renderView()
  await screen.findByText('Service is repeatedly crashing')

  await fireEvent.click(screen.getByRole('button', { name: 'False positive' }))
  await waitFor(() => expect(api.reviewHypothesis).toHaveBeenCalledWith('diagnosis-1', 'crash-loop', 'false_positive'))
  expect(await screen.findByRole('status')).toHaveTextContent('stored locally')

  await fireEvent.click(screen.getByRole('button', { name: 'Save disabled' }))
  await waitFor(() => expect(api.saveRecipe).toHaveBeenCalledWith(expect.objectContaining({ projectId: 'alpha', actionId: 'tests', maxRunsPerDay: 3 })))
  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('saved disabled'))
})

test('acknowledges a diagnostic alert locally', async () => {
  api.loadNotifications.mockResolvedValue([{ id: 'notification-1', projectId: 'alpha', code: 'REPEATED_CRASH', title: 'API keeps crashing', detail: 'Four restarts observed.', occurrences: 2, firstSeenAt: '2026-07-16T15:00:00Z', lastSeenAt: '2026-07-16T15:20:00Z' }])
  renderView()
  expect(await screen.findByText('API keeps crashing')).toBeInTheDocument()
  await fireEvent.click(screen.getByRole('button', { name: 'Acknowledge' }))
  await waitFor(() => expect(api.acknowledgeNotification).toHaveBeenCalledWith('notification-1'))
  expect(await screen.findByRole('status')).toHaveTextContent('acknowledged locally')
})
