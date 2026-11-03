import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ClientsManager } from './App'
import { api } from './api'
import type { Customer } from './types'

vi.mock('./api', () => ({
  api: { createCustomer: vi.fn(), updateCustomer: vi.fn(), customersPage: vi.fn() },
}))

const mockedApi = vi.mocked(api, true)

beforeEach(() => {
  vi.clearAllMocks()
  mockedApi.customersPage.mockResolvedValue({ items: [], total: 0 })
})

const existing: Customer = { id: 'c1', name: 'Cliente', active: true }

describe('ClientsManager', () => {
  it('loads the first page on mount', async () => {
    mockedApi.customersPage.mockResolvedValue({ items: [existing], total: 1 })
    render(<ClientsManager onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    expect(await screen.findByText('Cliente')).toBeInTheDocument()
    expect(mockedApi.customersPage).toHaveBeenCalledWith(1, 20)
  })

  it('creates a customer and reloads the current page', async () => {
    const created = { id: 'c2', name: 'Novo', active: true }
    mockedApi.createCustomer.mockResolvedValue(created)
    const onCreated = vi.fn()
    render(<ClientsManager onCreated={onCreated} onUpdated={vi.fn()} onBack={vi.fn()} />)
    await waitFor(() => expect(mockedApi.customersPage).toHaveBeenCalledTimes(1))

    await userEvent.click(screen.getByRole('button', { name: 'Novo cliente' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'Novo')
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar cliente' }))

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(created))
    await waitFor(() => expect(mockedApi.customersPage).toHaveBeenCalledTimes(2))
  })

  it('rejects an invalid e-mail without calling the API', async () => {
    render(<ClientsManager onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)
    await waitFor(() => expect(mockedApi.customersPage).toHaveBeenCalled())

    await userEvent.click(screen.getByRole('button', { name: 'Novo cliente' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'Novo')
    await userEvent.type(screen.getByPlaceholderText('E-mail (opcional)'), 'not-an-email')
    fireEvent.submit(screen.getByPlaceholderText('Nome').closest('form')!)

    expect(await screen.findByText('Informe um e-mail válido ou deixe em branco.')).toBeInTheDocument()
    expect(mockedApi.createCustomer).not.toHaveBeenCalled()
  })

  it('toggles a customer to inactive', async () => {
    mockedApi.customersPage.mockResolvedValue({ items: [existing], total: 1 })
    const updated = { ...existing, active: false }
    mockedApi.updateCustomer.mockResolvedValue(updated)
    const onUpdated = vi.fn()
    render(<ClientsManager onCreated={vi.fn()} onUpdated={onUpdated} onBack={vi.fn()} />)

    await userEvent.click(await screen.findByText('Cliente'))
    await userEvent.click(screen.getByRole('button', { name: 'Excluir' }))

    await waitFor(() => expect(onUpdated).toHaveBeenCalledWith(updated))
  })

  it('navigates to the next page', async () => {
    const pageOne = Array.from({ length: 20 }, (_, i) => ({ id: `c${i}`, name: `Cliente ${i}`, active: true }))
    const pageTwo = [{ id: 'c99', name: 'Última Cliente', active: true }]
    mockedApi.customersPage.mockImplementation(async (page: number) =>
      page === 1 ? { items: pageOne, total: 21 } : { items: pageTwo, total: 21 })
    render(<ClientsManager onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    expect(await screen.findByText('Página 1 de 2')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Próxima página' }))

    expect(await screen.findByText('Última Cliente')).toBeInTheDocument()
    expect(mockedApi.customersPage).toHaveBeenCalledWith(2, 20)
    expect(screen.getByText('Página 2 de 2')).toBeInTheDocument()
  })

  it('has no pagination controls for a single page', async () => {
    mockedApi.customersPage.mockResolvedValue({ items: [existing], total: 1 })
    render(<ClientsManager onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await screen.findByText('Cliente')
    expect(screen.queryByText(/Página \d de \d/)).not.toBeInTheDocument()
  })
})
