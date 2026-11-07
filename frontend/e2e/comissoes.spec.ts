import { test, expect } from '@playwright/test'
import { signUpTenant, loginViaUI, unique } from './fixtures'

test.use({ timezoneId: 'America/Sao_Paulo' })

// Noon on the 1st of the current month (São Paulo): always inside the default
// report period (current month), whatever day the suite runs.
function firstOfMonthAtNoon() {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-01T12:00:00-03:00`
}

test.describe('Comissões', () => {
  test('gestor define 40%, conclui um atendimento e o relatório e o profissional mostram R$ 20,00', async ({ page, browser, request }) => {
    const manager = await signUpTenant(request)
    const headers = { Authorization: `Bearer ${manager.accessToken}` }
    const call = async (method: 'post' | 'patch', path: string, data: unknown) => {
      const response = await request[method](`/api/v1${path}`, { headers, data })
      expect(response.ok(), await response.text()).toBeTruthy()
      return response.json()
    }
    const proName = unique('Barbeiro')
    const phone = `+5511${String(Date.now() % 1_000_000_000).padStart(9, '8')}`
    const service = await call('post', '/services', { name: 'Corte', duration_minutes: 30, price_cents: 5000 })
    const pro = await call('post', '/professionals', { name: proName, phone })
    const customer = await call('post', '/customers', { name: 'Cliente Comissão' })

    // Regra cadastrada pela tela: Config > Profissionais > profissional > Comissão.
    await loginViaUI(page, manager.email, manager.password)
    await expect(page.getByText('Sua agenda')).toBeVisible()
    await page.locator('nav').getByText('Config').click()
    await page.getByText('Cadastro de profissionais').click()
    await page.getByText(proName).click()
    await page.getByRole('button', { name: 'Comissão' }).click()
    await page.getByLabel('Comissão de Corte').fill('40')
    await page.getByRole('button', { name: 'Salvar comissões' }).click()
    await expect(page.getByText('Comissões salvas.')).toBeVisible()

    const appointment = await call('post', '/appointments', { customer_id: customer.id, professional_id: pro.id, service_id: service.id, starts_at: firstOfMonthAtNoon() })
    await call('patch', `/appointments/${appointment.id}/status`, { status: 'confirmed' })
    await call('patch', `/appointments/${appointment.id}/status`, { status: 'completed' })

    // Gestor: Relatórios > Comissões.
    await page.getByRole('button', { name: 'Voltar' }).click()
    await page.locator('nav').getByText('Config').click()
    await page.getByText('Ocupação, faturamento e retenção').click()
    const section = page.getByRole('region', { name: 'Comissões' })
    await expect(section.getByText('Total a pagar:')).toContainText('R$ 20,00')
    await expect(section.getByText(proName)).toBeVisible()

    // Profissional: acesso manual e aba Comissões com o mesmo valor.
    const grant = await call('post', `/professionals/${pro.id}/credentials`, { channel: 'manual' })
    const proContext = await browser.newContext({ timezoneId: 'America/Sao_Paulo' })
    const proPage = await proContext.newPage()
    await proPage.goto(`/${manager.slug}`)
    await proPage.getByLabel('Celular').fill(phone.slice(3))
    await proPage.getByLabel('Senha', { exact: true }).fill(grant.password)
    await proPage.getByRole('button', { name: 'Entrar', exact: true }).click()
    await expect(proPage.getByText('Sua agenda')).toBeVisible()
    await proPage.locator('nav').getByText('Comissões').click()
    await expect(proPage.getByRole('heading', { name: 'Minhas comissões' })).toBeVisible()
    await expect(proPage.locator('.profile-info').getByText('R$ 20,00')).toBeVisible()

    // Sem login a API recusa (o 403 de cliente é coberto nos testes de handler).
    expect((await request.get('/api/v1/reports/commissions', { headers: { Authorization: 'Bearer invalido' } })).status()).toBe(401)
    await proContext.close()
  })
})
