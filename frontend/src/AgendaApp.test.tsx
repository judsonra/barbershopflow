import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { AgendaApp } from './App'
import { api } from './api'
import type { Appointment, Customer, Professional, Service, Tenant, User } from './types'

vi.mock('./api', () => ({
  api: {
    appointments: vi.fn(),
    services: vi.fn(),
    professionals: vi.fn(),
    tenant: vi.fn(),
    customers: vi.fn(),
    updateStatus: vi.fn(),
  },
}))

const mockedApi = vi.mocked(api, true)

const manager: User = { id: 'm1', name: 'Gestor', role: 'manager', active: true }
const client: User = { id: 'c1', name: 'Cliente', role: 'client', customer_id: 'cust-1', active: true }

const tenant: Tenant = {
  id: 't1', name: 'Barbearia', slug: 'barbearia', self_scheduling_enabled: true,
  auto_confirm_appointments: true, cancellation_window_hours: 0, active: true, created_at: new Date().toISOString(),
}
const service: Service = { id: 'svc-1', name: 'Corte', duration_minutes: 30, price_cents: 5000, active: true }
const professional: Professional = { id: 'p1', name: 'Barbeiro', active: true }
const customer: Customer = { id: 'cust-1', name: 'Cliente', active: true }
const appointment: Appointment = {
  id: 'a1', customer_id: 'cust-1', customer_name: 'Cliente', professional_id: 'p1', professional_name: 'Barbeiro',
  service_id: 'svc-1', service_name: 'Corte', starts_at: new Date().toISOString(),
  ends_at: new Date().toISOString(), status: 'scheduled', price_cents: 5000,
}

beforeEach(() => {
  vi.clearAllMocks()
  mockedApi.appointments.mockResolvedValue([appointment])
  mockedApi.services.mockResolvedValue([service])
  mockedApi.professionals.mockResolvedValue([professional])
  mockedApi.tenant.mockResolvedValue(tenant)
  mockedApi.customers.mockResolvedValue([customer])
})

describe('AgendaApp', () => {
  it('loads and displays the day agenda', async () => {
    render(<AgendaApp user={manager} onLogout={vi.fn()} />)

    expect(await screen.findByText('Cliente')).toBeInTheDocument()
    expect(screen.getByText('R$ 50,00')).toBeInTheDocument()
    expect(mockedApi.customers).toHaveBeenCalled()
  })

  it('skips loading customers for a client user', async () => {
    render(<AgendaApp user={client} onLogout={vi.fn()} />)
    await screen.findByText('Cliente')
    expect(mockedApi.customers).not.toHaveBeenCalled()
  })

  it('confirms an appointment and reloads the agenda', async () => {
    mockedApi.updateStatus.mockResolvedValue({ ...appointment, status: 'confirmed' })
    render(<AgendaApp user={manager} onLogout={vi.fn()} />)
    await screen.findByText('Cliente')

    await userEvent.click(screen.getByRole('button', { name: 'Confirmar' }))

    await waitFor(() => expect(mockedApi.updateStatus).toHaveBeenCalledWith('a1', 'confirmed'))
    await waitFor(() => expect(mockedApi.appointments).toHaveBeenCalledTimes(2))
  })

  it('filters the agenda by status', async () => {
    const other: Appointment = { ...appointment, id: 'a2', status: 'completed' }
    mockedApi.appointments.mockResolvedValue([appointment, other])
    render(<AgendaApp user={manager} onLogout={vi.fn()} />)
    await waitFor(() => expect(screen.getAllByText('Cliente')).toHaveLength(2))

    await userEvent.selectOptions(screen.getByLabelText('Filtrar por status'), 'completed')

    expect(screen.getAllByText('Cliente')).toHaveLength(1)
  })

  it('a client can only cancel their own appointment', async () => {
    render(<AgendaApp user={client} onLogout={vi.fn()} />)
    await screen.findByText('Cliente')

    expect(screen.getByRole('button', { name: 'Cancelar' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Confirmar' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Não compareceu' })).not.toBeInTheDocument()
  })
})
