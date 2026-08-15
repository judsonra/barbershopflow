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

  it('sends a PATCH to update a service', async () => {
    const body = { id: '1', name: 'Corte', duration_minutes: 30, price_cents: 5000, active: false }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status: 200 })))

    await expect(api.updateService('1', { name: 'Corte', duration_minutes: 30, price_cents: 5000, active: false })).resolves.toEqual(body)
    expect(fetch).toHaveBeenCalledWith('/api/v1/services/1', expect.objectContaining({ method: 'PATCH' }))
  })

  it('sends a PATCH to update a professional', async () => {
    const body = { id: '1', name: 'Rafael', active: false }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status: 200 })))

    await expect(api.updateProfessional('1', { name: 'Rafael', active: false })).resolves.toEqual(body)
    expect(fetch).toHaveBeenCalledWith('/api/v1/professionals/1', expect.objectContaining({ method: 'PATCH' }))
  })

  it('sends a PATCH to update a customer', async () => {
    const body = { id: '1', name: 'Maria', active: false }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status: 200 })))

    await expect(api.updateCustomer('1', { name: 'Maria', active: false })).resolves.toEqual(body)
    expect(fetch).toHaveBeenCalledWith('/api/v1/customers/1', expect.objectContaining({ method: 'PATCH' }))
  })

  it('searches customers across tenants with the query URL-encoded', async () => {
    const body = [{ id: '1', name: 'Maria', active: true, tenant_id: 't1', tenant_name: 'Barbearia A', tenant_slug: 'barbearia-a' }]
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status: 200 })))

    await expect(api.searchCustomersAdmin('(11) 9')).resolves.toEqual(body)
    expect(fetch).toHaveBeenCalledWith('/api/v1/admin/customers?q=(11)%209', expect.any(Object))
  })
})
