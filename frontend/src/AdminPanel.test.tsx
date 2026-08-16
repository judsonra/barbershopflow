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
const tenant: Tenant = { id: 't1', name: 'Barbearia', slug: 'barbearia', self_scheduling_enabled: false, auto_confirm_appointments: false, cancellation_window_hours: 0, active: true, created_at: '' }

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

  it('shows an empty state with no tenants', async () => {
    mockedApi.listTenantsAdmin.mockResolvedValue([])
    render(<AdminPanel user={super_} onImpersonate={vi.fn()} onLogout={vi.fn()} />)

    expect(await screen.findByText('Nenhuma barbearia')).toBeInTheDocument()
  })
})
