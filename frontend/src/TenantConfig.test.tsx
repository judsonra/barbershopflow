import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TenantConfig } from './App'
import { api } from './api'
import type { Tenant } from './types'

vi.mock('./api', () => ({
  api: {
    holidays: vi.fn(),
    updateTenant: vi.fn(),
    createHoliday: vi.fn(),
    deleteHoliday: vi.fn(),
  },
}))

const mockedApi = vi.mocked(api, true)

beforeEach(() => {
  vi.clearAllMocks()
  mockedApi.holidays.mockResolvedValue([])
})

const tenant: Tenant = {
  id: 't1', name: 'Barbearia', slug: 'barbearia', self_scheduling_enabled: false,
  auto_confirm_appointments: false, cancellation_window_hours: 0, active: true, created_at: new Date().toISOString(),
}

describe('TenantConfig', () => {
  it('disables auto-confirm until self-scheduling is enabled', async () => {
    render(<TenantConfig tenant={tenant} onSaved={vi.fn()} onBack={vi.fn()} />)
    await waitFor(() => expect(mockedApi.holidays).toHaveBeenCalled())

    const autoConfirm = screen.getByLabelText(/Confirmar automaticamente/)
    expect(autoConfirm).toBeDisabled()

    await userEvent.click(screen.getByLabelText(/Permitir que clientes sugiram/))
    expect(autoConfirm).toBeEnabled()
  })

  it('saves the updated settings', async () => {
    const updated = { ...tenant, self_scheduling_enabled: true, cancellation_window_hours: 12 }
    mockedApi.updateTenant.mockResolvedValue(updated)
    const onSaved = vi.fn()
    render(<TenantConfig tenant={tenant} onSaved={onSaved} onBack={vi.fn()} />)
    await waitFor(() => expect(mockedApi.holidays).toHaveBeenCalled())

    await userEvent.click(screen.getByLabelText(/Permitir que clientes sugiram/))
    const windowInput = screen.getByLabelText(/Prazo mínimo/)
    await userEvent.clear(windowInput)
    await userEvent.type(windowInput, '12')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(updated))
    expect(mockedApi.updateTenant).toHaveBeenCalledWith(expect.objectContaining({ cancellation_window_hours: 12, self_scheduling_enabled: true }))
  })

  it('rejects a negative cancellation window without calling the API', async () => {
    render(<TenantConfig tenant={tenant} onSaved={vi.fn()} onBack={vi.fn()} />)
    await waitFor(() => expect(mockedApi.holidays).toHaveBeenCalled())

    const windowInput = screen.getByLabelText(/Prazo mínimo/)
    await userEvent.clear(windowInput)
    await userEvent.type(windowInput, '-5')
    fireEvent.submit(windowInput.closest('form')!)

    expect(await screen.findByText('Prazo de cancelamento inválido.')).toBeInTheDocument()
    expect(mockedApi.updateTenant).not.toHaveBeenCalled()
  })

  it('adds and removes a holiday', async () => {
    const created = { id: 'h1', date: '2026-12-25', name: 'Natal' }
    mockedApi.createHoliday.mockResolvedValue(created)
    mockedApi.deleteHoliday.mockResolvedValue({ message: 'Feriado removido.' })
    render(<TenantConfig tenant={tenant} onSaved={vi.fn()} onBack={vi.fn()} />)
    await waitFor(() => expect(mockedApi.holidays).toHaveBeenCalled())

    const dateInput = screen.getByLabelText('Data')
    fireEvent.change(dateInput, { target: { value: '2026-12-25' } })
    await userEvent.type(screen.getByPlaceholderText('Nome (opcional, ex: Natal)'), 'Natal')
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar feriado' }))

    expect(await screen.findByText(/Natal/)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Remover' }))
    await waitFor(() => expect(mockedApi.deleteHoliday).toHaveBeenCalledWith('h1'))
  })
})
