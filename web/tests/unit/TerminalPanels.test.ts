import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { beforeEach, expect, test, vi } from 'vitest'

const terminalAPI = vi.hoisted(() => ({
  loadTerminalSessions: vi.fn(),
  startTerminalSession: vi.fn(),
  stopTerminalSession: vi.fn(),
  loadAgentSessions: vi.fn(),
  startAgentSession: vi.fn(),
}))

vi.mock('../../src/domains/terminal/api', () => terminalAPI)

import AgentSessionsPanel from '../../src/domains/terminal/components/AgentSessionsPanel.vue'
import TerminalPanel from '../../src/domains/terminal/components/TerminalPanel.vue'
import { terminalFontFamily } from '../../src/domains/terminal/composables/useTerminalPanel'

function plugins() {
  return [
    [
      VueQueryPlugin,
      { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
    ] as [typeof VueQueryPlugin, { queryClient: QueryClient }],
  ]
}

beforeEach(() => {
  terminalAPI.loadTerminalSessions.mockReset().mockResolvedValue([])
  terminalAPI.loadAgentSessions.mockReset().mockResolvedValue([])
  terminalAPI.startTerminalSession.mockReset()
  terminalAPI.startAgentSession.mockReset().mockResolvedValue({ id: 'terminal-agent' })
  terminalAPI.stopTerminalSession.mockReset()
})

test('terminal launcher exposes typed targets and honest detach persistence', async () => {
  render(TerminalPanel, {
    props: {
      projectId: 'alpha',
      services: ['api', 'database'],
      environments: [],
      externalAvailable: true,
      actions: [
        {
          id: 'console',
          name: 'Console',
          type: 'command',
          command: ['bin/console'],
          workingDirectory: '.',
          shell: false,
          captureOutput: false,
          risk: 'interactive',
          timeoutSeconds: 0,
        },
      ],
    },
    global: { plugins: plugins() },
  })
  expect(await screen.findByRole('heading', { name: 'Interactive terminal' })).toBeInTheDocument()
  expect(screen.getByText(/Disconnecting detaches the browser/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Open external terminal' })).toBeEnabled()
  expect(screen.getByLabelText('Shell')).toHaveValue('')
  await fireEvent.update(screen.getByLabelText('Launch'), 'database')
  expect(screen.getByLabelText('Service')).toBeRequired()
  expect(screen.getByLabelText('Client')).toHaveValue('psql')
  await fireEvent.update(screen.getByLabelText('Launch'), 'action')
  expect(screen.getByRole('option', { name: 'Console' })).toBeInTheDocument()
})

test('embedded terminal prefers the Powerlevel10k font with portable fallbacks', () => {
  expect(terminalFontFamily).toBe(
    '"MesloLGS NF", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
  )
})

test('agent sessions disclose the observable boundary and launch a selected provider', async () => {
  render(AgentSessionsPanel, {
    props: { projectId: 'alpha', environments: [] },
    global: { plugins: plugins() },
  })
  expect(await screen.findByRole('heading', { name: 'Coding agents' })).toBeInTheDocument()
  expect(
    screen.getByText(/neither requests nor claims access to hidden reasoning/),
  ).toBeInTheDocument()
  await fireEvent.update(screen.getByLabelText('Provider'), 'claude')
  await fireEvent.click(screen.getByRole('button', { name: 'Start agent' }))
  await waitFor(() =>
    expect(terminalAPI.startAgentSession).toHaveBeenCalledWith({
      projectId: 'alpha',
      provider: 'claude',
      environmentId: undefined,
      columns: 120,
      rows: 36,
    }),
  )
})
