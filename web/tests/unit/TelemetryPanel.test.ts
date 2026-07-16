import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

const api = vi.hoisted(() => ({
  disableTelemetry: vi.fn(),
  enableTelemetry: vi.fn(),
  loadTelemetryStatus: vi.fn(),
  sendTelemetry: vi.fn(),
}))
vi.mock('../../src/domains/telemetry/api', () => api)
import TelemetryPanel from '../../src/domains/telemetry/components/TelemetryPanel.vue'

const disabled = {
  settings: { enabled: false, updatedAt: '2026-07-16T12:00:00Z' },
  counters: [],
}

afterEach(cleanup)
beforeEach(() => {
  vi.clearAllMocks()
  api.loadTelemetryStatus.mockResolvedValue(disabled)
  api.enableTelemetry.mockResolvedValue({
    settings: { enabled: true, endpoint: 'https://metrics.example.test/v1', installationId: 'anonymous-1', updatedAt: '2026-07-16T12:00:00Z' },
    counters: [],
  })
})

function renderPanel() {
  return render(TelemetryPanel, {
    global: {
      plugins: [[VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }) }]],
    },
  })
}

test('is off by default and requires review of the exact endpoint', async () => {
  renderPanel()
  expect(await screen.findByText('off by default')).toBeInTheDocument()
  expect(await screen.findByText('No counters are pending.')).toBeInTheDocument()
  expect(screen.getByText(/never projects, paths, logs, commands, or machine identities/i)).toBeInTheDocument()
  const button = screen.getByRole('button', { name: 'Enable anonymous counters' })
  expect(button).toBeDisabled()
  await fireEvent.update(screen.getByLabelText('HTTPS collection endpoint'), 'https://metrics.example.test/v1')
  await fireEvent.click(screen.getByRole('checkbox', { name: /reviewed the exact payload/i }))
  await fireEvent.click(button)
  await waitFor(() => expect(api.enableTelemetry).toHaveBeenCalledWith('https://metrics.example.test/v1'))
  expect(await screen.findByText('opted in')).toBeInTheDocument()
})
