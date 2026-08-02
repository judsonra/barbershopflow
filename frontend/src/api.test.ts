import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'

function memoryStorage(): Storage {
  const store = new Map<string, string>()
  return {
    getItem: (key) => store.get(key) ?? null,
    setItem: (key, value) => void store.set(key, value),
    removeItem: (key) => void store.delete(key),
    clear: () => store.clear(),
    key: (index) => Array.from(store.keys())[index] ?? null,
    get length() { return store.size }
  }
}

describe('api client', () => {
  beforeEach(() => vi.stubGlobal('localStorage', memoryStorage()))
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
