import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue'

import type { RuntimeLogEntry } from '../../../api/generated/types.gen'

export type LogConnectionState = 'connecting' | 'connected' | 'disconnected'

export function useProjectLogStream(
  projectId: Ref<string>,
  onEntry: (entry: RuntimeLogEntry) => void,
) {
  const state = ref<LogConnectionState>('connecting')
  const lastSequence = ref(0)
  let socket: WebSocket | undefined
  let reconnectTimer: number | undefined
  let attempts = 0
  let disposed = false

  function connect() {
    if (typeof WebSocket === 'undefined') {
      state.value = 'disconnected'
      return
    }
    state.value = attempts === 0 ? 'connecting' : 'disconnected'
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const query = new URLSearchParams({
      projectId: projectId.value,
      after: String(lastSequence.value),
    })
    socket = new WebSocket(`${protocol}//${window.location.host}/ws/v1/logs?${query}`)
    socket.addEventListener('message', (message) => {
      try {
        const event = JSON.parse(String(message.data)) as Partial<RuntimeLogEntry> & {
          type?: string
        }
        if (event.type === 'logs.connected') {
          state.value = 'connected'
          attempts = 0
          return
        }
        if (typeof event.sequence !== 'number' || typeof event.message !== 'string')
          throw new Error()
        if (event.sequence > lastSequence.value) {
          lastSequence.value = event.sequence
          onEntry(event as RuntimeLogEntry)
        }
        state.value = 'connected'
        attempts = 0
      } catch {
        socket?.close(1002, 'invalid log envelope')
      }
    })
    socket.addEventListener('close', () => {
      state.value = 'disconnected'
      if (!disposed) {
        reconnectTimer = window.setTimeout(connect, Math.min(5_000, 250 * 2 ** attempts))
        attempts += 1
      }
    })
    socket.addEventListener('error', () => {
      state.value = 'disconnected'
      socket?.close()
    })
  }

  onMounted(connect)
  onBeforeUnmount(() => {
    disposed = true
    if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer)
    socket?.close()
  })

  return { state, lastSequence }
}
