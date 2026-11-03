import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TenantAccount } from './App'
import { api } from './api'
import type { Tenant } from './types'

vi.mock('./api', () => ({ api: { checkTenantName: vi.fn(), updateTenant: vi.fn() } }))

const mockedApi = vi.mocked(api, true)

beforeEach(() => { vi.clearAllMocks() })

const tenant: Tenant = {
  id: 't1', name: 'Barbearia Original', slug: 'barbearia-original', self_scheduling_enabled: false,
  auto_confirm_appointments: false, cancellation_window_hours: 0, timezone: 'America/Sao_Paulo', active: true, created_at: new Date().toISOString(),
}

describe('TenantAccount', () => {
  it('saves the updated name and slug', async () => {
    const updated = { ...tenant, name: 'Novo Nome', slug: 'novo-nome' }
    mockedApi.updateTenant.mockResolvedValue(updated)
    const onSaved = vi.fn()
    render(<TenantAccount tenant={tenant} onSaved={onSaved} onBack={vi.fn()} />)

    const nameInput = screen.getByLabelText(/Nome da barbearia/)
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'Novo Nome')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(updated))
  })

  it('saves the selected timezone', async () => {
    mockedApi.updateTenant.mockResolvedValue({ ...tenant, timezone: 'America/Manaus' })
    render(<TenantAccount tenant={tenant} onSaved={vi.fn()} onBack={vi.fn()} />)

    expect(screen.getByLabelText(/Fuso horário/)).toHaveValue('America/Sao_Paulo')
    await userEvent.selectOptions(screen.getByLabelText(/Fuso horário/), 'America/Manaus')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))

    await waitFor(() => expect(mockedApi.updateTenant).toHaveBeenCalledWith(expect.objectContaining({ timezone: 'America/Manaus' })))
  })

  it('keeps a timezone set outside the Brazilian list', () => {
    render(<TenantAccount tenant={{ ...tenant, timezone: 'Europe/Lisbon' }} onSaved={vi.fn()} onBack={vi.fn()} />)
    expect(screen.getByLabelText(/Fuso horário/)).toHaveValue('Europe/Lisbon')
  })

  it('does not check availability when the name is unchanged', async () => {
    render(<TenantAccount tenant={tenant} onSaved={vi.fn()} onBack={vi.fn()} />)
    await new Promise(resolve => setTimeout(resolve, 450))
    expect(mockedApi.checkTenantName).not.toHaveBeenCalled()
  })

  it('checks availability when the name changes', async () => {
    mockedApi.checkTenantName.mockResolvedValue({ available: false })
    render(<TenantAccount tenant={tenant} onSaved={vi.fn()} onBack={vi.fn()} />)

    const nameInput = screen.getByLabelText(/Nome da barbearia/)
    await userEvent.type(nameInput, ' Filial')

    expect(await screen.findByText('Nome já em uso')).toBeInTheDocument()
    expect(mockedApi.checkTenantName).toHaveBeenCalled()
  })
})
