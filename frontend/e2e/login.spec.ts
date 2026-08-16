import { test, expect } from '@playwright/test'
import { SEED_SUPERADMIN, loginViaUI, unique } from './fixtures'

test.describe('Login', () => {
  test('signs up a new barbershop and lands on the agenda', async ({ page }) => {
    const tenantName = unique('E2E Barbearia')
    const email = `${unique('manager')}@example.com`

    await page.goto('/')
    await page.getByRole('button', { name: 'Cadastrar minha barbearia' }).click()
    await page.getByLabel('Nome da barbearia').fill(tenantName)
    await page.getByLabel('Seu nome').fill('Gestor E2E')
    await page.getByLabel('E-mail').fill(email)
    await page.getByLabel('Senha').fill('senha12345')
    await page.getByRole('button', { name: 'Criar barbearia' }).click()

    await expect(page.getByText('Sua agenda')).toBeVisible()
  })

  test('shows an error for the wrong password', async ({ page }) => {
    await loginViaUI(page, SEED_SUPERADMIN.email, 'wrong-password')
    await expect(page.getByRole('alert')).toContainText('inválidos')
  })
})
