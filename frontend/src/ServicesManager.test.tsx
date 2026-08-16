import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ServicesManager } from './App'
import { api } from './api'
import type { Service } from './types'

vi.mock('./api', () => ({
  api: {
    createService: vi.fn(),
    updateService: vi.fn(),
  },
}))

const mockedApi = vi.mocked(api, true)

beforeEach(() => {
  vi.clearAllMocks()
})

const existingService: Service = { id: 's1', name: 'Corte', duration_minutes: 30, price_cents: 5000, active: true }

describe('ServicesManager', () => {
  it('lists existing services', () => {
    render(<ServicesManager services={[existingService]} onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)
    expect(screen.getByText('Corte')).toBeInTheDocument()
    expect(screen.getByText(/30 min/)).toBeInTheDocument()
  })

  it('creates a service, converting comma-decimal price to cents', async () => {
    const created = { id: 's2', name: 'Barba', duration_minutes: 20, price_cents: 3500, active: true }
    mockedApi.createService.mockResolvedValue(created)
    const onCreated = vi.fn()
    render(<ServicesManager services={[]} onCreated={onCreated} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Novo serviço' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'Barba')
    const durationInput = screen.getByPlaceholderText('Duração (minutos)')
    await userEvent.clear(durationInput)
    await userEvent.type(durationInput, '20')
    await userEvent.type(screen.getByPlaceholderText('Preço (R$)'), '35,00')
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar serviço' }))

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(created))
    expect(mockedApi.createService).toHaveBeenCalledWith({ name: 'Barba', duration_minutes: 20, price_cents: 3500 })
  })

  // These two cases exercise submit()'s own JS-level checks, which run
  // *in addition to* the inputs' native HTML5 constraints (min/max,
  // required) — the same defense-in-depth the signup form uses for its
  // password length, since some browsers silently swallow the native
  // validation bubble. fireEvent.submit dispatches the submit event
  // directly, bypassing the click-triggered native constraint check, so the
  // JS-level branch can be exercised on its own.
  it('rejects a duration below 5 minutes without calling the API', async () => {
    const { container } = render(<ServicesManager services={[]} onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Novo serviço' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'Barba')
    const durationInput = screen.getByPlaceholderText('Duração (minutos)')
    await userEvent.clear(durationInput)
    await userEvent.type(durationInput, '2')
    await userEvent.type(screen.getByPlaceholderText('Preço (R$)'), '10,00')
    fireEvent.submit(container.querySelector('form')!)

    expect(await screen.findByText('Duração deve ser entre 5 e 480 minutos.')).toBeInTheDocument()
    expect(mockedApi.createService).not.toHaveBeenCalled()
  })

  it('rejects a non-numeric price without calling the API', async () => {
    const { container } = render(<ServicesManager services={[]} onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Novo serviço' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'Barba')
    await userEvent.type(screen.getByPlaceholderText('Preço (R$)'), 'abc')
    fireEvent.submit(container.querySelector('form')!)

    expect(await screen.findByText('Informe um preço válido.')).toBeInTheDocument()
    expect(mockedApi.createService).not.toHaveBeenCalled()
  })

  it('toggles a service to inactive', async () => {
    const updated = { ...existingService, active: false }
    mockedApi.updateService.mockResolvedValue(updated)
    const onUpdated = vi.fn()
    render(<ServicesManager services={[existingService]} onCreated={vi.fn()} onUpdated={onUpdated} onBack={vi.fn()} />)

    await userEvent.click(screen.getByText('Corte'))
    await userEvent.click(screen.getByRole('button', { name: 'Excluir' }))

    await waitFor(() => expect(onUpdated).toHaveBeenCalledWith(updated))
    expect(mockedApi.updateService).toHaveBeenCalledWith('s1', { name: 'Corte', duration_minutes: 30, price_cents: 5000, active: false })
  })
})
