import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { NewAppointment } from './App'
import { api } from './api'
import type { Appointment, Customer, Professional, Service, Tenant, User } from './types'

vi.mock('./api', () => ({
  api: { createAppointment: vi.fn(), createCustomer: vi.fn(), grantCustomerAccess: vi.fn() },
}))

const mockedApi = vi.mocked(api, true)

beforeEach(() => { vi.clearAllMocks() })

const manager: User = { id: 'm1', name: 'Gestor', role: 'manager', active: true }
const client: User = { id: 'c1', name: 'Cliente', role: 'client', customer_id: 'cust-1', active: true }
const services: Service[] = [{ id: 'svc-1', name: 'Corte', duration_minutes: 30, price_cents: 5000, active: true }]
const professionals: Professional[] = [{ id: 'p1', name: 'Barbeiro', active: true }]
const customers: Customer[] = [{ id: 'cust-1', name: 'Cliente', active: true }]

describe('NewAppointment - staff booking', () => {
  it('books an appointment for an existing customer', async () => {
    const created: Appointment = {
      id: 'a1', customer_id: 'cust-1', customer_name: 'Cliente', professional_id: 'p1', professional_name: 'Barbeiro',
      service_id: 'svc-1', service_name: 'Corte', starts_at: '2026-06-01T10:00:00.000Z', ends_at: '2026-06-01T10:30:00.000Z',
      status: 'scheduled', price_cents: 5000,
    }
    mockedApi.createAppointment.mockResolvedValue(created)
    const onDone = vi.fn().mockResolvedValue(undefined)
    const tenant: Tenant = { id: 't1', name: 'B', slug: 'b', self_scheduling_enabled: false, auto_confirm_appointments: false, cancellation_window_hours: 0, active: true, created_at: '' }
    render(<NewAppointment user={manager} tenant={tenant} services={services} professionals={professionals} customers={customers} onDone={onDone} />)

    await userEvent.selectOptions(screen.getByLabelText('Cliente'), 'cust-1')
    await userEvent.selectOptions(screen.getByLabelText('Serviço'), 'svc-1')
    await userEvent.selectOptions(screen.getByLabelText('Profissional'), 'p1')
    await userEvent.type(screen.getByLabelText('Data e hora'), '2026-06-01T10:00')
    await userEvent.click(screen.getByRole('button', { name: 'Confirmar agendamento' }))

    await waitFor(() => expect(mockedApi.createAppointment).toHaveBeenCalled())
    await waitFor(() => expect(onDone).toHaveBeenCalled())
  })
})

describe('NewAppointment - client self-scheduling', () => {
  it('shows pending messaging when auto-confirm is off', () => {
    const tenant: Tenant = { id: 't1', name: 'B', slug: 'b', self_scheduling_enabled: true, auto_confirm_appointments: false, cancellation_window_hours: 0, active: true, created_at: '' }
    render(<NewAppointment user={client} tenant={tenant} services={services} professionals={professionals} customers={[]} onDone={vi.fn()} />)

    expect(screen.getByText(/precisa confirmar antes do horário ficar garantido/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Cliente')).not.toBeInTheDocument()
  })

  it('shows immediate-confirmation messaging when auto-confirm is on', () => {
    const tenant: Tenant = { id: 't1', name: 'B', slug: 'b', self_scheduling_enabled: true, auto_confirm_appointments: true, cancellation_window_hours: 0, active: true, created_at: '' }
    render(<NewAppointment user={client} tenant={tenant} services={services} professionals={professionals} customers={[]} onDone={vi.fn()} />)

    expect(screen.getByText(/reservado assim que você confirma/)).toBeInTheDocument()
  })
})
