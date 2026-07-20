import type { RuntimeLogEntry } from '../../src/api/generated/types.gen'
import { boundLogEntries } from '../../src/domains/logs/api'
import { expect, test } from 'vitest'

test('bounds and orders large log histories for responsive rendering', () => {
  const entries = Array.from({ length: 1_000 }, (_, index): RuntimeLogEntry => ({
    sequence: index,
    timestamp: new Date(Date.UTC(2026, 6, 15, 12, 0, index)).toISOString(),
    projectId: 'project-1',
    serviceId: 'api',
    runId: 'run-1',
    source: 'process',
    stream: 'stdout',
    level: 'info',
    message: `line ${index}`,
    redacted: false,
    attributes: {},
  }))

  const originalOrder = entries.map((entry) => entry.sequence)
  const result = boundLogEntries(entries)

  expect(result).toHaveLength(500)
  expect(result[0]?.sequence).toBe(999)
  expect(result.at(-1)?.sequence).toBe(500)
  expect(entries.map((entry) => entry.sequence)).toEqual(originalOrder)
})
