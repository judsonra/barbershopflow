import { test, expect } from '@playwright/test'
import { signUpTenant, loginViaUI, SEED_SUPERADMIN } from './fixtures'

test.describe('Navegação por papel', () => {
  test('gestor vê Config e Horários na navegação', async ({ page, request }) => {
    const manager = await signUpTenant(request)
    await loginViaUI(page, manager.email, manager.password)
    await expect(page.getByText('Sua agenda')).toBeVisible()

    const nav = page.locator('nav')
    await expect(nav.getByText('Config')).toBeVisible()
    await expect(nav.getByText('Horários')).toBeVisible()
  })

  test('superadmin vê o painel de barbearias, não a agenda de uma barbearia', async ({ page }) => {
    await loginViaUI(page, SEED_SUPERADMIN.email, SEED_SUPERADMIN.password)
    await expect(page.getByText('SUPERADMIN')).toBeVisible()

    const nav = page.locator('nav')
    await expect(nav.getByText('Barbearias')).toBeVisible()
    await expect(nav.getByText('Clientes')).toBeVisible()
    await expect(nav.getByText('Auditoria')).toBeVisible()
    await expect(page.getByText('Sua agenda')).not.toBeVisible()
  })
})
