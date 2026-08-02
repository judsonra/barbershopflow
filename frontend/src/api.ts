import type { Appointment, Customer, Professional, Service, User } from './types'

const ACCESS_KEY = 'bf_access_token'
const REFRESH_KEY = 'bf_refresh_token'

let sessionExpiredHandler: (() => void) | null = null

export class ApiError extends Error {
  code?: string
  constructor(message: string, code?: string) {
    super(message)
    this.code = code
  }
}

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
    throw new ApiError('Sessão expirada. Faça login novamente.')
  }
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    throw new ApiError(body?.error?.message || 'Não foi possível concluir a operação.', body?.error?.code)
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

// Google/Facebook finish with a full-page redirect back to `/`, carrying the
// tokens in the URL fragment (never sent to a server, unlike a query string).
// Call this once on app boot to pick them up.
function consumeOAuthRedirect(): { error?: string } {
  if (!location.hash) return {}
  const params = new URLSearchParams(location.hash.slice(1))
  const accessToken = params.get('access_token')
  const refreshToken = params.get('refresh_token')
  const error = params.get('auth_error')
  if (accessToken && refreshToken) storeTokens(accessToken, refreshToken)
  if (accessToken || error) history.replaceState(null, '', location.pathname + location.search)
  return { error: error ?? undefined }
}

export const api = {
  isAuthenticated: () => Boolean(getAccessToken()),
  onSessionExpired: (handler: () => void) => { sessionExpiredHandler = handler },
  consumeOAuthRedirect,
  socialLoginUrl: (provider: 'google' | 'facebook') => `/api/v1/auth/${provider}/start`,
  login: async (email: string, password: string) => {
    const data = await request<{ access_token: string; refresh_token: string; user: User }>(
      '/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }
    )
    storeTokens(data.access_token, data.refresh_token)
    return data.user
  },
  createTenant: async (data: { tenant_name: string; slug: string; manager_name: string; email: string; password: string }) => {
    const result = await request<{ access_token: string; refresh_token: string; user: User }>(
      '/tenants', { method: 'POST', body: JSON.stringify(data) }
    )
    storeTokens(result.access_token, result.refresh_token)
    return result.user
  },
  loginByPhone: async (phone: string, password: string) => {
    const data = await request<{ access_token: string; refresh_token: string; user: User }>(
      '/auth/login', { method: 'POST', body: JSON.stringify({ phone, password }) }
    )
    storeTokens(data.access_token, data.refresh_token)
    return data.user
  },
  recoverPhone: (phone: string, channel: 'sms' | 'whatsapp' = 'whatsapp') =>
    request<{ message: string }>('/auth/recover', { method: 'POST', body: JSON.stringify({ phone, channel }) }),
  grantCustomerAccess: (customerId: string, phone: string, channel: 'sms' | 'whatsapp' = 'whatsapp') =>
    request<{ message: string }>(`/customers/${customerId}/credentials`, { method: 'POST', body: JSON.stringify({ phone, channel }) }),
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
