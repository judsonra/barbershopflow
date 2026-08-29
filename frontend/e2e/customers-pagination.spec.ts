import type { APIRequestContext } from '@playwright/test'
import { test, expect } from '@playwright/test'
import { signUpTenant, loginViaUI, unique } from './fixtures'

// Must match CLIENTS_PAGE_SIZE in frontend/src/App.tsx.
const PAGE_SIZE = 20

async function seedCustomers(request: APIRequestContext, accessToken: string, count: number) {
  await Promise.all(Array.from({ length: count }, (_, i) => {
    const n = String(i + 1).padStart(2, '0')
    return request.post('/api/v1/customers', {
      headers: { Authorization: `Bearer ${accessToken}` },
      data: { name: `Cliente E2E ${n}` },
    })
  }))
}

test.describe('Paginação de clientes (Config → Clientes)', () => {
  test('navega entre páginas quando há mais clientes que o tamanho da página', async ({ page, request }) => {
    const tenant = await signUpTenant(request, unique('Paginação Barbearia'))
    await seedCustomers(request, tenant.accessToken, PAGE_SIZE + 2)

    await loginViaUI(page, tenant.email, tenant.password)
    await expect(page.getByText('Sua agenda')).toBeVisible()

    await page.getByRole('button', { name: 'Config' }).click()
    await page.getByText('Cadastro de clientes').click()

    await expect(page.getByText('Página 1 de 2')).toBeVisible()
    await expect(page.getByText(`${PAGE_SIZE + 2} cliente(s)`)).toBeVisible()
    await expect(page.getByText('Cliente E2E 01')).toBeVisible()
    await expect(page.getByText('Cliente E2E 22')).not.toBeVisible()
    await expect(page.getByRole('button', { name: 'Página anterior' })).toBeDisabled()

    await page.getByRole('button', { name: 'Próxima página' }).click()

    await expect(page.getByText('Página 2 de 2')).toBeVisible()
    await expect(page.getByText('Cliente E2E 22')).toBeVisible()
    await expect(page.getByText('Cliente E2E 01')).not.toBeVisible()
    await expect(page.getByRole('button', { name: 'Próxima página' })).toBeDisabled()

    await page.getByRole('button', { name: 'Página anterior' }).click()

    await expect(page.getByText('Página 1 de 2')).toBeVisible()
    await expect(page.getByText('Cliente E2E 01')).toBeVisible()
  })

  test('esconde os controles de paginação quando cabe tudo em uma página', async ({ page, request }) => {
    const tenant = await signUpTenant(request, unique('Página Única Barbearia'))
    await seedCustomers(request, tenant.accessToken, 3)

    await loginViaUI(page, tenant.email, tenant.password)
    await expect(page.getByText('Sua agenda')).toBeVisible()

    await page.getByRole('button', { name: 'Config' }).click()
    await page.getByText('Cadastro de clientes').click()

    await expect(page.getByText('Cliente E2E 03')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Próxima página' })).not.toBeVisible()
    await expect(page.getByRole('button', { name: 'Página anterior' })).not.toBeVisible()
  })
})
