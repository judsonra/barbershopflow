import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { AdminPanel } from './App'
import { api } from './api'
import type { Tenant, User } from './types'

vi.mock('./api', () => ({
  api: { listTenantsAdmin: vi.fn(), searchCustomersAdmin: vi.fn(), listImpersonationAudit: vi.fn(), impersonateTenant: vi.fn() },
}))

const mockedApi = vi.mocked(api, true)

beforeEach(() => { vi.clearAllMocks() })

const super_: User = { id: 's1', name: 'Super', role: 'superadmin', active: true }
const tenant: Tenant = { id: 't1', name: 'Barbearia', slug: 'barbearia', self_scheduling_enabled: false, auto_confirm_appointments: false, cancellation_window_hours: 0, timezone: 'America/Sao_Paulo', active: true, created_at: '' }

describe('AdminPanel', () => {
  it('lists tenants and impersonates one', async () => {
    mockedApi.listTenantsAdmin.mockResolvedValue([tenant])
    const impersonated: User = { id: 'm1', name: 'Gestor', role: 'manager', active: true }
    mockedApi.impersonateTenant.mockResolvedValue(impersonated)
    const onImpersonate = vi.fn()
    render(<AdminPanel user={super_} onImpersonate={onImpersonate} onLogout={vi.fn()} />)

    expect(await screen.findByText('Barbearia')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Acessar como gestor' }))

    await waitFor(() => expect(onImpersonate).toHaveBeenCalledWith(impersonated))
    expect(mockedApi.impersonateTenant).toHaveBeenCalledWith('t1')
  })

  it('filters tenants by name or slug, ignoring case and accents', async () => {
    mockedApi.listTenantsAdmin.mockResolvedValue([
      { ...tenant, id: 't1', name: 'Junin Barber Shop', slug: 'junin' },
      { ...tenant, id: 't2', name: 'Barbearia do João', slug: 'joao-centro' },
      { ...tenant, id: 't3', name: 'Corte Fino', slug: 'corte-fino' },
    ])
    render(<AdminPanel user={super_} onImpersonate={vi.fn()} onLogout={vi.fn()} />)
    expect(await screen.findByText('Junin Barber Shop')).toBeInTheDocument()
    const search = screen.getByRole('searchbox', { name: 'Pesquisar barbearia' })

    await userEvent.type(search, 'JOAO')
    expect(screen.getByText('Barbearia do João')).toBeInTheDocument()
    expect(screen.queryByText('Junin Barber Shop')).not.toBeInTheDocument()
    expect(screen.getByText('1 de 3 barbearia(s)')).toBeInTheDocument()

    await userEvent.clear(search)
    await userEvent.type(search, 'corte-f')
    expect(screen.getByText('Corte Fino')).toBeInTheDocument()
    expect(screen.queryByText('Barbearia do João')).not.toBeInTheDocument()

    await userEvent.clear(search)
    await userEvent.type(search, 'inexistente')
    expect(screen.getByText('Nenhuma barbearia encontrada')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Acessar como gestor' })).not.toBeInTheDocument()

    await userEvent.clear(search)
    expect(screen.getAllByRole('button', { name: 'Acessar como gestor' })).toHaveLength(3)
  })

  it('shows an empty state with no tenants', async () => {
    mockedApi.listTenantsAdmin.mockResolvedValue([])
    render(<AdminPanel user={super_} onImpersonate={vi.fn()} onLogout={vi.fn()} />)

    expect(await screen.findByText('Nenhuma barbearia')).toBeInTheDocument()
  })
})
