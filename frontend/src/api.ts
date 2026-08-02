import type { Appointment, Customer, Professional, Service, User } from './types'

const ACCESS_KEY = 'bf_access_token'
const REFRESH_KEY = 'bf_refresh_token'

let sessionExpiredHandler: (() => void) | null = null

function getAccessToken() { return localStorage.getItem(ACCESS_KEY) }
function getRefreshToken() { return localStorage.getItem(REFRESH_KEY) }
function storeTokens(accessToken: string, refreshToken: string) {
  localStorage.setItem(ACCESS_KEY, accessToken)
  localStorage.setItem(REFRESH_KEY, refreshToken)
}
function clearTokens() {
  localStorage.removeItem(ACCESS_KEY)
  localStorage.removeItem(REFRESH_KEY)
}

async function request<T>(path: string, options?: RequestInit, allowRefresh = true): Promise<T> {
  const token = getAccessToken()
  const response = await fetch(`/api/v1${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options?.headers
    }
  })
  if (response.status === 401 && allowRefresh && path !== '/auth/login' && path !== '/auth/refresh') {
    if (await tryRefresh()) {
      return request<T>(path, options, false)
    }
    clearTokens()
    sessionExpiredHandler?.()
    throw new Error('Sessão expirada. Faça login novamente.')
  }
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    throw new Error(body?.error?.message || 'Não foi possível concluir a operação.')
  }
  if (response.status === 204) return undefined as T
  return response.json()
}

async function tryRefresh(): Promise<boolean> {
  const refreshToken = getRefreshToken()
  if (!refreshToken) return false
  try {
    const data = await request<{ access_token: string; refresh_token: string }>(
      '/auth/refresh', { method: 'POST', body: JSON.stringify({ refresh_token: refreshToken }) }, false
    )
    storeTokens(data.access_token, data.refresh_token)
    return true
  } catch {
    return false
  }
}

export const api = {
  isAuthenticated: () => Boolean(getAccessToken()),
  onSessionExpired: (handler: () => void) => { sessionExpiredHandler = handler },
  login: async (email: string, password: string) => {
    const data = await request<{ access_token: string; refresh_token: string; user: User }>(
      '/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }
    )
    storeTokens(data.access_token, data.refresh_token)
    return data.user
  },
  logout: () => clearTokens(),
  me: () => request<User>('/auth/me'),
  services: () => request<Service[]>('/services'),
  professionals: () => request<Professional[]>('/professionals'),
  customers: () => request<Customer[]>('/customers'),
  createCustomer: (data: Omit<Customer, 'id'>) => request<Customer>('/customers', { method: 'POST', body: JSON.stringify(data) }),
  appointments: (from: string, to: string) => request<Appointment[]>(`/appointments?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`),
  createAppointment: (data: object) => request<Appointment>('/appointments', { method: 'POST', body: JSON.stringify(data) }),
  updateStatus: (id: string, status: Appointment['status']) =>
    request<Appointment>(`/appointments/${id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) })
}
