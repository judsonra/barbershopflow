import type { APIRequestContext, Page } from '@playwright/test'
import { expect } from '@playwright/test'

// Seeded by backend/internal/database/migrations/002_users.sql +
// 009_superadmin_seed.sql: the only account that starts out as superadmin
// in a fresh dev database, needed to reach /admin/promote and impersonation.
export const SEED_SUPERADMIN = { email: 'admin@barberflow.local', password: 'change-me' }

export function unique(prefix: string) {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 100000)}`
}

export type SignedUpManager = { tenantName: string; slug: string; email: string; password: string; accessToken: string }

// Uses the public self-serve signup endpoint directly (not the UI) so
// specs that aren't specifically testing the signup form itself can get a
// fresh, isolated tenant+manager in one call.
export async function signUpTenant(request: APIRequestContext, tenantName = unique('E2E Barbearia')): Promise<SignedUpManager> {
  const slug = unique('e2e')
  const email = `${unique('manager')}@example.com`
  const password = 'senha12345'
  const response = await request.post('/api/v1/tenants', {
    data: { tenant_name: tenantName, slug, manager_name: 'Gestor E2E', email, password },
  })
  expect(response.ok(), await response.text()).toBeTruthy()
  const body = await response.json()
  return { tenantName, slug, email, password, accessToken: body.access_token }
}

export async function apiLogin(request: APIRequestContext, email: string, password: string): Promise<string> {
  const response = await request.post('/api/v1/auth/login', { data: { email, password } })
  expect(response.ok(), await response.text()).toBeTruthy()
  const body = await response.json()
  return body.access_token as string
}

export async function loginViaUI(page: Page, email: string, password: string) {
  await page.goto('/')
  await page.getByLabel('E-mail').fill(email)
  await page.getByLabel('Senha').fill(password)
  await page.getByRole('button', { name: 'Entrar' }).click()
}
