import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

const settingsAPI = vi.hoisted(() => ({ loadDaemonSettings: vi.fn(), saveDaemonSettings: vi.fn() }))
vi.mock('../../src/domains/system/settingsApi', async () => ({ ...settingsAPI, SettingsAPIError: class extends Error {} }))
vi.mock('../../src/domains/system/api', () => ({
  loadSystemInfo: vi.fn().mockResolvedValue({ status: 'ready', version: '1.0.0', commit: 'fixture', builtAt: '2026-07-16T12:00:00Z', apiVersion: 'v1', databaseSchemaVersion: 17, startedAt: '2026-07-16T11:00:00Z' }),
  loadHostObservation: vi.fn().mockResolvedValue({ cpuPercent: 8, memoryUsedBytes: 1024, memoryTotalBytes: 4096, docker: { connected: true, storageBytes: 0, reclaimableBytes: 0, attribution: 'shared' }, observedAt: '2026-07-16T12:00:00Z', warnings: [] }),
}))
vi.mock('../../src/domains/telemetry/api', () => ({
  loadTelemetryStatus: vi.fn().mockResolvedValue({ settings: { enabled: false, updatedAt: '2026-07-16T12:00:00Z' }, counters: [] }),
  enableTelemetry: vi.fn(), disableTelemetry: vi.fn(), sendTelemetry: vi.fn(),
}))

import SettingsView from '../../src/domains/system/views/SettingsView.vue'

const current = {
  settings: {
    revision: 1,
    projectRoots: ['/Users/dev/projects'],
    ports: { rangeStart: 15000, rangeEnd: 19999, excluded: [] },
    retention: { logAgeSeconds: 604800, logMaximumBytes: 268435456, metricRawSeconds: 3600, metricMinuteSeconds: 86400, metricQuarterHourSeconds: 2592000, maximumMetricHistoryPoints: 1000 },
    tools: { terminal: 'integrated' as const, editor: 'vscode' as const },
    ai: { defaultProvider: 'codex' as const, providers: [
      { id: 'codex' as const, enabled: true, executable: 'codex' },
      { id: 'claude' as const, enabled: true, executable: 'claude' },
      { id: 'openai-compatible' as const, enabled: false },
    ] as const },
    permissions: { defaultAgentProfile: 'observe' as const },
    appearance: { density: 'comfortable' as const, timeDisplay: 'relative' as const, theme: 'dark' as const },
    updatedAt: '2026-07-16T12:00:00Z',
  },
  pendingRestart: [] as Array<'retention' | 'ai.providers'>,
}

afterEach(cleanup)
beforeEach(() => {
  vi.clearAllMocks()
  settingsAPI.loadDaemonSettings.mockResolvedValue(structuredClone(current))
  settingsAPI.saveDaemonSettings.mockImplementation(async (settings) => ({ settings: { ...settings, revision: 2, updatedAt: '2026-07-16T12:01:00Z' }, pendingRestart: ['retention'] }))
})

test('edits one revision and reports restart-bound settings honestly', async () => {
  render(SettingsView, { global: { plugins: [[VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }) }]] } })
  expect(await screen.findByRole('heading', { name: 'Project roots' })).toBeInTheDocument()
  expect(screen.getByText('Revision 1')).toBeInTheDocument()
  const save = screen.getByRole('button', { name: 'Save settings' })
  expect(save).toBeDisabled()
  await fireEvent.update(screen.getByLabelText(/Log age/), '8')
  expect(save).toBeEnabled()
  await fireEvent.click(save)
  await waitFor(() => expect(settingsAPI.saveDaemonSettings).toHaveBeenCalled())
  expect(await screen.findByText(/Restart the daemon to apply/)).toHaveTextContent('retention')
  expect(screen.getByText('Revision 2')).toBeInTheDocument()
})
