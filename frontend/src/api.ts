import type { Appointment, Customer, Professional, Service } from './types'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options?.headers }
  })
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    throw new Error(body?.error?.message || 'Não foi possível concluir a operação.')
  }
  return response.json()
}

export const api = {
  services: () => request<Service[]>('/services'),
  professionals: () => request<Professional[]>('/professionals'),
  customers: () => request<Customer[]>('/customers'),
  createCustomer: (data: Omit<Customer, 'id'>) => request<Customer>('/customers', { method: 'POST', body: JSON.stringify(data) }),
  appointments: (from: string, to: string) => request<Appointment[]>(`/appointments?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`),
  createAppointment: (data: object) => request<Appointment>('/appointments', { method: 'POST', body: JSON.stringify(data) }),
  updateStatus: (id: string, status: Appointment['status']) =>
    request<Appointment>(`/appointments/${id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) })
}
