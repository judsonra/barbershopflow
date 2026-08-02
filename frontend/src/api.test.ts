import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'

describe('api client', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('surfaces the business error returned by the API', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(
      JSON.stringify({ error: { message: 'Horário indisponível' } }),
      { status: 409, headers: { 'Content-Type': 'application/json' } }
    )))

    await expect(api.createAppointment({})).rejects.toThrow('Horário indisponível')
  })

  it('loads services from the versioned endpoint', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('[]', { status: 200 })))

    await expect(api.services()).resolves.toEqual([])
    expect(fetch).toHaveBeenCalledWith('/api/v1/services', expect.any(Object))
  })
})
