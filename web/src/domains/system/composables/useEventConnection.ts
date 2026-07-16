import { onBeforeUnmount, onMounted, ref } from 'vue'

type ConnectionState = 'connecting' | 'connected' | 'disconnected'

export function useEventConnection() {
  const state = ref<ConnectionState>('connecting')
  let socket: WebSocket | undefined

  onMounted(() => {
    if (typeof WebSocket === 'undefined') {
      state.value = 'disconnected'
      return
    }
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    socket = new WebSocket(`${protocol}//${window.location.host}/ws/v1/events`)
    socket.addEventListener('message', () => {
      state.value = 'connected'
    })
    socket.addEventListener('close', () => {
      state.value = 'disconnected'
    })
    socket.addEventListener('error', () => {
      state.value = 'disconnected'
    })
  })

  onBeforeUnmount(() => socket?.close())
  return state
}
