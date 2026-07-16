import { render, screen } from '@testing-library/vue'
import { defineComponent, nextTick, ref } from 'vue'
import { afterEach, expect, test, vi } from 'vitest'

import type { RuntimeLogEntry } from '../../src/api/generated/types.gen'
import { useProjectLogStream } from '../../src/domains/projects/composables/useProjectLogStream'

type Listener = (event: MessageEvent | Event) => void

class FakeWebSocket {
  static instances: Array<FakeWebSocket> = []
  readonly listeners = new Map<string, Array<Listener>>()

  constructor(readonly url: string) {
    FakeWebSocket.instances.push(this)
  }

  addEventListener(type: string, listener: Listener) {
    const listeners = this.listeners.get(type) ?? []
    listeners.push(listener)
    this.listeners.set(type, listeners)
  }

  emit(type: string, data?: unknown) {
    const event = type === 'message' ? new MessageEvent('message', { data: JSON.stringify(data) }) : new Event(type)
    for (const listener of this.listeners.get(type) ?? []) listener(event)
  }

  close() {}
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  FakeWebSocket.instances = []
})

test('reconnects from the last sequence and ignores replay overlap', async () => {
  vi.useFakeTimers()
  vi.stubGlobal('WebSocket', FakeWebSocket)
  const received: Array<number> = []
  const component = defineComponent({
    setup() {
      const stream = useProjectLogStream(ref('project-1'), (entry: RuntimeLogEntry) => received.push(entry.sequence))
      return { state: stream.state }
    },
    template: '<span>{{ state }}</span>',
  })
  const view = render(component)
  const first = FakeWebSocket.instances[0]!
  expect(first.url).toContain('projectId=project-1&after=0')
  first.emit('message', { type: 'logs.connected', sequence: 0 })
  first.emit('message', logEntry(1))
  first.emit('message', logEntry(1))
  expect(received).toEqual([1])
  first.emit('close')
  await nextTick()
  expect(screen.getByText('disconnected')).toBeInTheDocument()

  await vi.advanceTimersByTimeAsync(250)
  const second = FakeWebSocket.instances[1]!
  expect(second.url).toContain('after=1')
  second.emit('message', logEntry(1))
  second.emit('message', logEntry(2))
  expect(received).toEqual([1, 2])
  view.unmount()
})

function logEntry(sequence: number): RuntimeLogEntry {
  return { sequence, timestamp: '2026-07-16T12:00:00Z', projectId: 'project-1', serviceId: 'api', runId: 'run-1', source: 'process', stream: 'stdout', level: 'info', message: `line ${sequence}`, redacted: false, attributes: {} }
}
