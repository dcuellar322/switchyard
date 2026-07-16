import { beforeEach, describe, expect, it, vi } from 'vitest'

const { createBrowserSession, setConfig } = vi.hoisted(() => ({
  createBrowserSession: vi.fn(),
  setConfig: vi.fn(),
}))

vi.mock('../../src/api/generated/sdk.gen', () => ({ createBrowserSession }))
vi.mock('../../src/api/generated/client.gen', () => ({ client: { setConfig } }))

import { bootstrapBrowserSession, mutationHeaders } from '../../src/domains/session/bootstrap'

describe('browser session bootstrap', () => {
  beforeEach(() => {
    createBrowserSession.mockReset()
    setConfig.mockReset()
    window.sessionStorage.clear()
    window.history.replaceState({}, '', '/?bootstrap=boot_test')
  })

  it('exchanges the URL token, stores CSRF state, and removes the secret from history', async () => {
    createBrowserSession.mockResolvedValue({ data: { csrfToken: 'csrf_test' } })

    await bootstrapBrowserSession()

    expect(createBrowserSession).toHaveBeenCalledWith({
      body: { bootstrapToken: 'boot_test' },
      credentials: 'same-origin',
    })
    expect(window.location.search).toBe('')
    expect(mutationHeaders('request-key')).toEqual({
      'Idempotency-Key': 'request-key',
      'X-CSRF-Token': 'csrf_test',
    })
    expect(setConfig).toHaveBeenCalledWith({ credentials: 'same-origin' })
  })
})
