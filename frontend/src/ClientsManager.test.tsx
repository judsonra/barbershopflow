import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ClientsManager } from './App'
import { api } from './api'
import type { Customer } from './types'

vi.mock('./api', () => ({
  api: { createCustomer: vi.fn(), updateCustomer: vi.fn() },
}))

const mockedApi = vi.mocked(api, true)

beforeEach(() => { vi.clearAllMocks() })

const existing: Customer = { id: 'c1', name: 'Cliente', active: true }

describe('ClientsManager', () => {
  it('creates a customer', async () => {
    const created = { id: 'c2', name: 'Novo', active: true }
    mockedApi.createCustomer.mockResolvedValue(created)
    const onCreated = vi.fn()
    render(<ClientsManager customers={[]} onCreated={onCreated} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Novo cliente' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'Novo')
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar cliente' }))

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(created))
  })

  it('rejects an invalid e-mail without calling the API', async () => {
    render(<ClientsManager customers={[]} onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Novo cliente' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'Novo')
    await userEvent.type(screen.getByPlaceholderText('E-mail (opcional)'), 'not-an-email')
    fireEvent.submit(screen.getByPlaceholderText('Nome').closest('form')!)

    expect(await screen.findByText('Informe um e-mail válido ou deixe em branco.')).toBeInTheDocument()
    expect(mockedApi.createCustomer).not.toHaveBeenCalled()
  })

  it('toggles a customer to inactive', async () => {
    const updated = { ...existing, active: false }
    mockedApi.updateCustomer.mockResolvedValue(updated)
    const onUpdated = vi.fn()
    render(<ClientsManager customers={[existing]} onCreated={vi.fn()} onUpdated={onUpdated} onBack={vi.fn()} />)

    await userEvent.click(screen.getByText('Cliente'))
    await userEvent.click(screen.getByRole('button', { name: 'Excluir' }))

    await waitFor(() => expect(onUpdated).toHaveBeenCalledWith(updated))
  })
})
