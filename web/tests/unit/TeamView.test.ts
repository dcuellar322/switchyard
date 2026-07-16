import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

const api = vi.hoisted(() => ({
  installBundle: vi.fn(),
  loadCuratedPlugins: vi.fn(),
  loadEffectivePolicy: vi.fn(),
  loadTeamBundles: vi.fn(),
  loadTeamPublishers: vi.fn(),
  trustPublisher: vi.fn(),
}))
vi.mock('../../src/domains/team/api', () => api)
import TeamView from '../../src/domains/team/views/TeamView.vue'

const publisher = {
  id: `publisher-${'a'.repeat(32)}`,
  name: 'Switchyard maintainers',
  publicKey: 'cHVibGlj',
  trustedAt: '2026-07-16T12:00:00Z',
}

afterEach(cleanup)
beforeEach(() => {
  vi.clearAllMocks()
  api.loadTeamPublishers.mockResolvedValue([publisher])
  api.loadTeamBundles.mockResolvedValue([])
  api.loadEffectivePolicy.mockResolvedValue({
    sourceBundleIds: ['policy.official'],
    allowedRemoteCapabilities: ['inventory.read'],
    allowedRemoteActions: ['start'],
    allowedPluginPublishers: [publisher.id],
    telemetryAllowed: false,
    requireSignedConfiguration: true,
  })
  api.loadCuratedPlugins.mockResolvedValue([{
    id: 'reviewed-plugin', name: 'Reviewed plugin', version: '1.0.0', summary: 'Signed metadata.',
    publisher: publisher.id, downloadUrl: 'https://plugins.example.test/plugin.tar.gz',
    sha256: 'b'.repeat(64), platforms: ['linux/amd64'], capabilities: ['project.inspect'],
  }])
  api.trustPublisher.mockResolvedValue(publisher)
})

function renderView() {
  return render(TeamView, {
    global: {
      plugins: [[VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }) }]],
    },
  })
}

test('shows restrictive signed policy and keeps publisher trust behind review', async () => {
  renderView()
  expect(await screen.findByText('Switchyard maintainers')).toBeInTheDocument()
  expect(await screen.findByText('Reviewed plugin')).toBeInTheDocument()
  expect(screen.getByText('denied')).toBeInTheDocument()
  const button = screen.getByRole('button', { name: 'Trust exact key' })
  expect(button).toBeDisabled()
  await fireEvent.update(screen.getByLabelText('Name'), 'Maintainers')
  await fireEvent.update(screen.getByLabelText('Base64 Ed25519 public key'), 'cHVibGlj')
  await fireEvent.click(screen.getByRole('checkbox', { name: /verified this exact key/i }))
  await fireEvent.click(button)
  await waitFor(() => expect(api.trustPublisher).toHaveBeenCalledWith('Maintainers', 'cHVibGlj'))
})

test('explains that encrypted sync excludes sensitive local state', async () => {
  renderView()
  expect(await screen.findByRole('heading', { name: 'Encrypted sync' })).toBeInTheDocument()
  expect(screen.getByText(/Fleet credentials, projects, paths, logs, operations, and secrets are never included/)).toBeInTheDocument()
})
