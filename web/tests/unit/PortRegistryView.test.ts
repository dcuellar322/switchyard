import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { fireEvent, render, screen } from '@testing-library/vue'
import { expect, test, vi } from 'vitest'

vi.mock('../../src/domains/ports/api', () => ({
  loadPortRegistry: vi.fn().mockResolvedValue({
    observedAt: '2026-07-16T12:00:00Z', warnings: [],
    facts: [
      { id: 'decl', kind: 'declaration', projectId: 'project-a', projectName: 'Stopped App', serviceId: 'web', host: '127.0.0.1', port: 18081, protocol: 'tcp', source: 'manifest', evidence: 'accepted manifest', observedAt: '2026-07-16T12:00:00Z' },
      { id: 'bound', kind: 'binding', projectId: 'project-b', projectName: 'Running App', serviceId: 'web', host: '127.0.0.1', port: 18081, protocol: 'tcp', source: 'process', evidence: 'live listener', observedAt: '2026-07-16T12:00:00Z' },
    ],
    conflicts: [{ id: 'conflict', type: 'DECLARED_VS_BOUND', port: 18081, summary: 'conflict', facts: [] }],
  }),
  suggestPort: vi.fn().mockResolvedValue({ port: 18082, rangeStart: 15000, rangeEnd: 19999, protocol: 'tcp', observedAt: '2026-07-16T12:00:00Z' }),
}))

import PortRegistryView from '../../src/domains/ports/views/PortRegistryView.vue'

test('renders conflict provenance and a current free-port suggestion', async () => {
  render(PortRegistryView, {
    global: { plugins: [[VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })

  expect(await screen.findByRole('heading', { name: 'All port facts' })).toBeInTheDocument()
  expect(screen.getAllByText('18081/tcp')).toHaveLength(2)
  expect(screen.getAllByText('conflict').length).toBeGreaterThan(0)
  await fireEvent.click(screen.getByRole('button', { name: 'Find next free port' }))
  expect((await screen.findAllByText(/18082/)).length).toBeGreaterThan(0)
})
