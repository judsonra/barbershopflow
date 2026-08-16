import { test, expect } from '@playwright/test'
import { signUpTenant, loginViaUI } from './fixtures'

function datetimeLocalValue(date: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

// A fully client-driven self-scheduling flow would need a client-role
// login, which in this app only exists via phone (SMS/WhatsApp-delivered
// password, not retrievable from an E2E test) or OAuth (no provider
// configured in dev) — see docs/regras-de-negocio.md. This spec instead
// exercises the config toggle end-to-end from the manager's side: turning
// self-scheduling + auto-confirm on, then completing a booking through the
// same service/professional data a client would see.
test.describe('Autoagendamento', () => {
  test('gestor habilita autoagendamento e agenda um horário com o serviço/profissional cadastrados', async ({ page, request }) => {
    const manager = await signUpTenant(request)
    await loginViaUI(page, manager.email, manager.password)
    await expect(page.getByText('Sua agenda')).toBeVisible()

    // Enable self-scheduling + auto-confirm (ConfigMenu > "Agenda")
    await page.getByRole('button', { name: 'Config' }).click()
    await page.getByText('Autoagendamento e confirmação').click()
    await page.getByLabel(/Permitir que clientes sugiram/).check()
    await page.getByLabel(/Confirmar automaticamente/).check()
    await page.getByRole('button', { name: 'Salvar' }).click()
    await expect(page.getByText('O horário do cliente já entra confirmado e reservado.')).toBeVisible()
    await page.getByRole('button', { name: 'Voltar' }).click()

    // Add a service
    await page.getByText('Cadastro de serviços').click()
    await page.getByRole('button', { name: 'Novo serviço' }).click()
    await page.getByPlaceholder('Nome').fill('Corte E2E')
    await page.getByPlaceholder('Duração (minutos)').fill('30')
    await page.getByPlaceholder('Preço (R$)').fill('50,00')
    await page.getByRole('button', { name: 'Adicionar serviço' }).click()
    await expect(page.getByText('Corte E2E')).toBeVisible()
    await page.getByRole('button', { name: 'Voltar' }).click()

    // Add a professional
    await page.getByText('Cadastro de profissionais').click()
    await page.getByRole('button', { name: 'Novo profissional' }).click()
    await page.getByPlaceholder('Nome').fill('Barbeiro E2E')
    await page.getByRole('button', { name: 'Adicionar profissional' }).click()
    await expect(page.getByText('Barbeiro E2E')).toBeVisible()

    // Book an appointment later today, with a quick-created customer
    await page.locator('nav button', { hasText: 'Agenda' }).click()
    await page.locator('nav button', { hasText: 'Novo' }).click()
    await page.locator('details summary').click()
    await page.getByPlaceholder('Nome').fill('Cliente E2E')
    await page.getByRole('button', { name: 'Adicionar' }).click()
    await page.getByLabel('Serviço').selectOption({ label: 'Corte E2E · R$ 50,00' })
    await page.getByLabel('Profissional').selectOption({ label: 'Barbeiro E2E' })
    await page.getByLabel('Data e hora').fill(datetimeLocalValue(new Date(Date.now() + 2 * 60 * 60 * 1000)))
    await page.getByRole('button', { name: 'Confirmar agendamento' }).click()

    await expect(page.getByText('Cliente E2E')).toBeVisible()
    await expect(page.getByText('Corte E2E · Barbeiro E2E')).toBeVisible()
  })
})
