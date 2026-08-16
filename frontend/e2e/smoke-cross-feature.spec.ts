import { test, expect } from '@playwright/test'
import { signUpTenant, loginViaUI, apiLogin, SEED_SUPERADMIN, unique } from './fixtures'

function datetimeLocalValue(date: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

// The broadest scenario: a manager sets up especialidades (a professional's
// service+price override), a tenant holiday, books and marks a no-show —
// then a superadmin promotes a *different* tenant's manager and impersonates
// the first tenant to confirm its data (the no-show, the overridden price)
// is visible from there. Deliberately last/slowest of the E2E specs, since
// it walks the full stack across features the unit/component layers below
// can't observe interacting together.
test.describe('Fluxo cruzado: especialidades, feriado, no-show, promote e impersonate', () => {
  test('gestor configura especialidades/feriado/no-show; superadmin promovido impersona e vê os dados', async ({ page, request }) => {
    const tenantA = await signUpTenant(request, unique('Smoke Barbearia'))
    await loginViaUI(page, tenantA.email, tenantA.password)
    await expect(page.getByText('Sua agenda')).toBeVisible()

    // Service + professional
    await page.getByRole('button', { name: 'Config' }).click()
    await page.getByText('Cadastro de serviços').click()
    await page.getByRole('button', { name: 'Novo serviço' }).click()
    await page.getByPlaceholder('Nome').fill('Corte Especial')
    await page.getByPlaceholder('Duração (minutos)').fill('40')
    await page.getByPlaceholder('Preço (R$)').fill('80,00')
    await page.getByRole('button', { name: 'Adicionar serviço' }).click()
    await expect(page.getByText('Corte Especial')).toBeVisible()
    await page.getByRole('button', { name: 'Voltar' }).click()

    await page.getByText('Cadastro de profissionais').click()
    await page.getByRole('button', { name: 'Novo profissional' }).click()
    await page.getByPlaceholder('Nome').fill('Barbeiro Especialista')
    await page.getByRole('button', { name: 'Adicionar profissional' }).click()
    await expect(page.getByText('Barbeiro Especialista')).toBeVisible()

    // Especialidades e preços: only this service, with a price override
    await page.getByText('Barbeiro Especialista').click()
    await page.getByRole('button', { name: 'Especialidades e preços' }).click()
    await page.getByRole('checkbox', { name: /Corte Especial/ }).check()
    await page.getByPlaceholder('Preço (R$, opcional)').fill('70,00')
    await page.getByRole('button', { name: 'Salvar especialidades' }).click()
    await page.getByRole('button', { name: 'Voltar' }).click() // detail -> list
    await page.getByRole('button', { name: 'Voltar' }).click() // list -> config menu

    // Tenant holiday
    await page.getByText('Autoagendamento e confirmação').click()
    const holidayDate = new Date(Date.now() + 10 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10)
    await page.getByLabel('Data').fill(holidayDate)
    await page.getByPlaceholder('Nome (opcional, ex: Natal)').fill('Feriado E2E')
    await page.getByRole('button', { name: 'Adicionar feriado' }).click()
    await expect(page.getByText('Feriado E2E')).toBeVisible()
    await page.getByRole('button', { name: 'Voltar' }).click()

    // Book (today, not the holiday) and mark as no-show
    await page.locator('nav button', { hasText: 'Agenda' }).click()
    await page.locator('nav button', { hasText: 'Novo' }).click()
    await page.locator('details summary').click()
    await page.getByPlaceholder('Nome').fill('Cliente Smoke')
    await page.getByRole('button', { name: 'Adicionar' }).click()
    await page.getByLabel('Serviço').selectOption({ label: 'Corte Especial · R$ 80,00' })
    await page.getByLabel('Profissional').selectOption({ label: 'Barbeiro Especialista' })
    await page.getByLabel('Data e hora').fill(datetimeLocalValue(new Date(Date.now() + 2 * 60 * 60 * 1000)))
    await page.getByRole('button', { name: 'Confirmar agendamento' }).click()

    await expect(page.getByText('Cliente Smoke')).toBeVisible()
    // The specialty's overridden price (70,00), not the service's own (80,00).
    await expect(page.getByText('R$ 70,00')).toBeVisible()
    const card = page.locator('article.card', { hasText: 'Cliente Smoke' })
    await card.getByRole('button', { name: 'Não compareceu' }).click()
    await expect(card.locator('.card-title span')).toHaveText('Não compareceu')

    // Promote a *different* tenant's manager - promoting tenantA's own
    // manager would leave tenantA with no manager at all, breaking
    // impersonation for it (GetFirstManagerByTenant -> ErrNotFound).
    const tenantB = await signUpTenant(request, unique('Smoke Outro'))
    const superToken = await apiLogin(request, SEED_SUPERADMIN.email, SEED_SUPERADMIN.password)
    const promoteResp = await request.post('/api/v1/admin/promote', {
      headers: { Authorization: `Bearer ${superToken}` },
      data: { email: tenantB.email },
    })
    expect(promoteResp.ok(), await promoteResp.text()).toBeTruthy()

    // Log out and back in as the now-promoted account to pick up a fresh
    // token with role=superadmin (the old browser session's token still
    // carries the stale role baked in at mint time).
    await page.locator('button.avatar').click()
    await page.getByRole('button', { name: 'Sair' }).click()
    await loginViaUI(page, tenantB.email, tenantB.password)
    await expect(page.getByText('SUPERADMIN')).toBeVisible()

    const row = page.locator('li', { hasText: tenantA.tenantName })
    await row.getByRole('button', { name: 'Acessar como gestor' }).click()

    await expect(page.getByText('Sua agenda')).toBeVisible()
    await expect(page.getByText('Cliente Smoke')).toBeVisible()
    await expect(page.getByText('R$ 70,00')).toBeVisible()
  })
})
