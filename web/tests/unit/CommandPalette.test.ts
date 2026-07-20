/* eslint-disable vue/one-component-per-file */
import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { defineComponent } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { expect, test, vi } from 'vitest'

vi.mock('../../src/domains/projects/api', () => ({
  loadProjects: vi
    .fn()
    .mockResolvedValue([{ id: 'alpha', displayName: 'Alpha App', slug: 'alpha', tags: [] }]),
  runProjectAction: vi.fn(),
  runRuntimeAction: vi.fn(),
}))

import CommandPalette from '../../src/app/components/CommandPalette.vue'

test('filters and executes navigation commands entirely from the keyboard', async () => {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: defineComponent({ template: '<div />' }) },
      {
        path: '/logs',
        component: defineComponent({ template: '<h1>Logs</h1>' }),
      },
      {
        path: '/:pathMatch(.*)*',
        component: defineComponent({ template: '<div />' }),
      },
    ],
  })
  await router.push('/')
  await router.isReady()
  const view = render(CommandPalette, {
    props: { open: true },
    global: {
      plugins: [
        router,
        [
          VueQueryPlugin,
          {
            queryClient: new QueryClient({
              defaultOptions: { queries: { retry: false } },
            }),
          },
        ],
      ],
    },
  })

  const input = screen.getByRole('textbox', {
    name: 'Type a command or project',
  })
  await fireEvent.update(input, 'error logs')
  expect(await screen.findByRole('option', { name: /Show all error logs/ })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  await fireEvent.keyDown(document, { key: 'Enter' })

  await waitFor(() => expect(router.currentRoute.value.fullPath).toBe('/logs?level=error'))
  expect(view.emitted().close).toHaveLength(1)
})
