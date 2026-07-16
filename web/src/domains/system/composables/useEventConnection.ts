import { onBeforeUnmount, onMounted, ref } from 'vue'

type ConnectionState = 'connecting' | 'connected' | 'disconnected'

export function useEventConnection() {
  const state = ref<ConnectionState>('connecting')
  let socket: WebSocket | undefined
  let reconnectTimer: number | undefined
  let lastSequence = 0
  let attempts = 0
  let disposed = false

  function connect() {
    if (typeof WebSocket === 'undefined') {
      state.value = 'disconnected'
      return
    }
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    socket = new WebSocket(`${protocol}//${window.location.host}/ws/v1/events?after=${lastSequence}`)
    socket.addEventListener('message', (message) => {
      try {
        const event = JSON.parse(String(message.data)) as { sequence?: number }
        if (typeof event.sequence === 'number') lastSequence = Math.max(lastSequence, event.sequence)
      } catch {
        socket?.close(1002, 'invalid event envelope')
        return
      }
      attempts = 0
      state.value = 'connected'
    })
    socket.addEventListener('close', () => {
      state.value = 'disconnected'
      if (!disposed) {
        const delay = Math.min(5_000, 250 * 2 ** attempts)
        attempts += 1
        reconnectTimer = window.setTimeout(connect, delay)
      }
    })
    socket.addEventListener('error', () => {
      state.value = 'disconnected'
      socket?.close()
    })
  }

  onMounted(() => {
    connect()
  })

  onBeforeUnmount(() => {
    disposed = true
    if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer)
    socket?.close()
  })
  return state
}
