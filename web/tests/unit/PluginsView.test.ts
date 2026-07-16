import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

const api = vi.hoisted(() => ({
  activatePlugin: vi.fn(), approvePlugin: vi.fn(), deactivatePlugin: vi.fn(), discoverPlugins: vi.fn(),
  loadPluginLogs: vi.fn(), loadPlugins: vi.fn(), probePlugin: vi.fn(),
}))
vi.mock('../../src/domains/plugins/api', () => api)
import PluginsView from '../../src/domains/plugins/views/PluginsView.vue'

const plugin = {
  id: 'fixture-inspector', name: 'Fixture inspector', version: '1.0.0', protocolVersion: 'switchyard.plugin/v1',
  manifestPath: '/plugins/fixture/plugin.json', fingerprint: 'a'.repeat(64), capabilities: ['project.inspect', 'project.operate'],
  requestedScopes: ['project.metadata.read', 'project.files.read', 'project.operate'], grantedScopes: [],
  available: true, enabled: false, trust: 'untrusted', health: 'unknown', discoveredAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
} as const

afterEach(cleanup)
beforeEach(() => { vi.clearAllMocks(); api.loadPlugins.mockResolvedValue([plugin]); api.loadPluginLogs.mockResolvedValue([]); api.approvePlugin.mockResolvedValue({ ...plugin, trust: 'trusted' }) })

function renderView() {
  return render(PluginsView, { global: { plugins: [[VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }) }]] } })
}

test('requires exact identity review and exposes requested scopes before trust', async () => {
  renderView()
  expect(await screen.findByRole('heading', { name: 'Fixture inspector' })).toBeInTheDocument()
  expect(screen.getAllByText('project.operate')).toHaveLength(2)
  const trust = screen.getByRole('button', { name: 'Trust exact fingerprint' })
  expect(trust).toBeDisabled()
  await fireEvent.click(screen.getByRole('checkbox', { name: /reviewed this exact fingerprint/i }))
  await fireEvent.click(trust)
  await waitFor(() => expect(api.approvePlugin).toHaveBeenCalledWith('fixture-inspector', 'a'.repeat(64)))
})

test('shows discovery empty and failure states without fake success', async () => {
  api.loadPlugins.mockResolvedValueOnce([])
  const empty = renderView()
  expect(await screen.findByRole('heading', { name: 'No plugins discovered' })).toBeInTheDocument()
  empty.unmount()
  api.loadPlugins.mockRejectedValueOnce(new Error('offline'))
  renderView()
  expect(await screen.findByRole('alert')).toHaveTextContent('Plugin registrations are unavailable')
})
