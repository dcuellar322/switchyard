import { describe, expect, test } from 'vitest'

import { tagTone } from '../../src/lib/format'

describe('tagTone', () => {
  test.each([
    ['frontend', 'purple'],
    ['marketing', 'purple'],
    ['backend', 'blue'],
    ['api', 'blue'],
    ['postgres', 'green'],
    ['redis', 'orange'],
    ['gateway', 'cyan'],
    ['custom', 'neutral'],
  ])('maps %s to the %s semantic tone', (tag, tone) => {
    expect(tagTone(tag)).toBe(tone)
  })
})
