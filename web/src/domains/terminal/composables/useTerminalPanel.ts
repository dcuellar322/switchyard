import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { Terminal } from '@xterm/xterm'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'

import type {
  ActionDefinition,
  ProjectEnvironment,
  TerminalSession,
  TerminalSessionKind,
} from '../../../api/generated/types.gen'
import { loadTerminalSessions, startTerminalSession, stopTerminalSession } from '../api'

export interface TerminalPanelProps {
  projectId: string
  services: Array<string>
  environments: Array<ProjectEnvironment>
  actions: Array<ActionDefinition>
  externalAvailable?: boolean
}

export function useTerminalPanel(props: TerminalPanelProps) {
  const queryClient = useQueryClient()
  const terminalHost = ref<HTMLElement>()
  const selectedKind = ref<TerminalSessionKind>('shell')
  const selectedShell = ref<'' | 'sh' | 'bash' | 'zsh'>('')
  const selectedService = ref('')
  const selectedEnvironment = ref('')
  const selectedProvider = ref<'codex' | 'claude'>('codex')
  const selectedDatabase = ref<'psql' | 'mysql' | 'redis-cli' | 'mongosh' | 'sqlite3'>('psql')
  const selectedAction = ref('')
  const attached = ref<TerminalSession>()
  const connectionState = ref<'idle' | 'connecting' | 'connected' | 'detached' | 'ended' | 'error'>(
    'idle',
  )
  const notice = ref('')
  let terminal: Terminal | undefined
  let fit: FitAddon | undefined
  let socket: WebSocket | undefined
  let observer: InstanceType<typeof window.ResizeObserver> | undefined
  let resizeTimer: number | undefined
  const sessions = useQuery({
    queryKey: computed(() => ['terminal-sessions', props.projectId]),
    queryFn: () => loadTerminalSessions(props.projectId),
    refetchInterval: 5_000,
  })
  const interactiveActions = computed(() =>
    props.actions.filter(
      (action) =>
        action.risk === 'interactive' &&
        !['terminal.open', 'agent.start', 'browser.open', 'editor.open'].includes(action.type),
    ),
  )
  const activeSessions = computed(
    () =>
      sessions.data.value?.filter((session) => ['starting', 'active'].includes(session.status)) ??
      [],
  )

  function safeOpen(event: { metaKey: boolean; ctrlKey: boolean }, uri: string) {
    if (!(event.metaKey || event.ctrlKey)) {
      notice.value = 'Hold Command or Control while opening terminal links.'
      return
    }
    try {
      const target = new window.URL(uri)
      if (!['http:', 'https:'].includes(target.protocol)) throw new Error('unsupported protocol')
      window.open(target.href, '_blank', 'noopener,noreferrer')
    } catch {
      notice.value = 'That terminal link was blocked.'
    }
  }
  function ensureTerminal() {
    if (terminal || !terminalHost.value) return
    fit = new FitAddon()
    terminal = new Terminal({
      cursorBlink: true,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 13,
      scrollback: 2_000,
      screenReaderMode: true,
      theme: {
        background: '#080c11',
        foreground: '#dce7f5',
        cursor: '#78a6ff',
        selectionBackground: '#29446f',
      },
      linkHandler: { activate: safeOpen },
    })
    terminal.loadAddon(fit)
    terminal.loadAddon(new WebLinksAddon(safeOpen))
    terminal.open(terminalHost.value)
    fit.fit()
    terminal.onData((data) => {
      if (socket?.readyState === WebSocket.OPEN) socket.send(new window.TextEncoder().encode(data))
    })
    observer = new window.ResizeObserver(() => scheduleResize())
    observer.observe(terminalHost.value)
  }
  function scheduleResize() {
    window.clearTimeout(resizeTimer)
    resizeTimer = window.setTimeout(() => {
      fit?.fit()
      if (socket?.readyState === WebSocket.OPEN && terminal) {
        socket.send(JSON.stringify({ type: 'resize', columns: terminal.cols, rows: terminal.rows }))
      }
    }, 80)
  }
  function disconnect() {
    if (!socket) return
    socket.onclose = null
    socket.close(1000, 'browser detached')
    socket = undefined
  }
  async function attach(session: TerminalSession) {
    disconnect()
    attached.value = session
    connectionState.value = 'connecting'
    notice.value = ''
    await nextTick()
    ensureTerminal()
    terminal?.clear()
    terminal?.writeln(`\x1b[90mConnecting to ${session.displayName}…\x1b[0m`)
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    socket = new WebSocket(
      `${protocol}//${window.location.host}/ws/v1/terminal/${encodeURIComponent(session.id)}`,
    )
    socket.binaryType = 'arraybuffer'
    socket.onopen = () => {
      connectionState.value = 'connected'
      scheduleResize()
      terminal?.focus()
    }
    socket.onmessage = (event) => {
      if (typeof event.data === 'string') {
        const control = JSON.parse(event.data) as { type: string; reason?: string; status?: string }
        if (control.type === 'exit') {
          connectionState.value = 'ended'
          notice.value =
            control.reason === 'slow_consumer_detached'
              ? 'Detached because this browser could not keep up with output.'
              : `Session ended (${control.status ?? 'finished'}).`
          void sessions.refetch()
        }
        return
      }
      terminal?.write(new Uint8Array(event.data as ArrayBuffer))
    }
    socket.onerror = () => {
      connectionState.value = 'error'
      notice.value = 'The terminal stream could not connect.'
    }
    socket.onclose = () => {
      if (!['ended', 'error'].includes(connectionState.value)) {
        connectionState.value = 'detached'
        notice.value =
          'Detached. The process remains available for 30 minutes unless you terminate it.'
      }
    }
  }
  const create = useMutation({
    mutationFn: async () =>
      startTerminalSession({
        projectId: props.projectId,
        environmentId: selectedEnvironment.value || undefined,
        kind: selectedKind.value,
        columns: terminal?.cols || 120,
        rows: terminal?.rows || 36,
        serviceId: ['service', 'database'].includes(selectedKind.value)
          ? selectedService.value
          : undefined,
        provider: selectedKind.value === 'agent' ? selectedProvider.value : undefined,
        databaseClient: selectedKind.value === 'database' ? selectedDatabase.value : undefined,
        actionId: selectedKind.value === 'action' ? selectedAction.value : undefined,
        shell: ['shell', 'service'].includes(selectedKind.value)
          ? selectedShell.value || undefined
          : undefined,
      }),
    onSuccess: async (session) => {
      await queryClient.invalidateQueries({ queryKey: ['terminal-sessions', props.projectId] })
      await attach(session)
    },
  })
  const terminate = useMutation({
    mutationFn: (sessionId: string) => stopTerminalSession(sessionId),
    onSuccess: async () => {
      notice.value = 'Termination requested.'
      await queryClient.invalidateQueries({ queryKey: ['terminal-sessions', props.projectId] })
    },
  })
  onBeforeUnmount(() => {
    disconnect()
    observer?.disconnect()
    terminal?.dispose()
    window.clearTimeout(resizeTimer)
  })
  return {
    terminalHost,
    selectedKind,
    selectedShell,
    selectedService,
    selectedEnvironment,
    selectedProvider,
    selectedDatabase,
    selectedAction,
    attached,
    connectionState,
    notice,
    sessions,
    interactiveActions,
    activeSessions,
    create,
    terminate,
    attach,
  }
}
