import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { render, screen } from '@testing-library/vue'
import { expect, test, vi } from 'vitest'

vi.mock('../../src/domains/system/api', () => ({
  loadSystemInfo: vi.fn().mockResolvedValue({
    status: 'ready',
    version: '0.1.0-test',
    commit: 'abc123',
    apiVersion: 'v1',
    databaseSchemaVersion: 1,
    startedAt: '2026-07-15T12:00:00Z',
  }),
}))

import App from '../../src/app/App.vue'

test('shows real generated system response data', async () => {
  render(App, {
    global: {
      plugins: [
        [
          VueQueryPlugin,
          { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
        ],
      ],
    },
  })

  expect(await screen.findByText('0.1.0-test')).toBeInTheDocument()
  expect(screen.getByText('1')).toBeInTheDocument()
})
