import { test, expect } from '@playwright/test'
import { signUpTenant, loginViaUI, unique } from './fixtures'

test.use({ timezoneId: 'America/Sao_Paulo' })

// Tomorrow at 10:00 in São Paulo, so the appointment never falls on the edge
// of "today" whatever time the suite runs.
function tomorrowAt10() {
  const day = new Date()
  day.setDate(day.getDate() + 1)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${day.getFullYear()}-${pad(day.getMonth() + 1)}-${pad(day.getDate())}T10:00:00-03:00`
}

function uniquePhone() {
  return `+5511${String(Date.now() % 1_000_000_000).padStart(9, '9')}`
}

test.describe('Acesso do profissional', () => {
  test('gestor gera a senha, o profissional entra, vê só a própria agenda e troca a senha', async ({ page, browser, request }) => {
    const manager = await signUpTenant(request)
    const headers = { Authorization: `Bearer ${manager.accessToken}` }
    const post = async (path: string, data: unknown) => {
      const response = await request.post(`/api/v1${path}`, { headers, data })
      expect(response.ok(), await response.text()).toBeTruthy()
      return response.json()
    }
    const phone = uniquePhone()
    const proName = unique('Barbeiro')
    const service = await post('/services', { name: 'Corte', duration_minutes: 30, price_cents: 5000 })
    const pro = await post('/professionals', { name: proName, phone })
    const other = await post('/professionals', { name: unique('Outro') })
    const mine = await post('/customers', { name: 'Cliente do Barbeiro' })
    const theirs = await post('/customers', { name: 'Cliente do Outro' })
    const startsAt = tomorrowAt10()
    await post('/appointments', { customer_id: mine.id, professional_id: pro.id, service_id: service.id, starts_at: startsAt })
    await post('/appointments', { customer_id: theirs.id, professional_id: other.id, service_id: service.id, starts_at: startsAt })

    // Gestor: Config > Profissionais > profissional > Conceder acesso > Gerar senha.
    await loginViaUI(page, manager.email, manager.password)
    await expect(page.getByText('Sua agenda')).toBeVisible()
    await page.locator('nav').getByText('Config').click()
    await page.getByText('Cadastro de profissionais').click()
    await page.getByText(proName).click()
    await page.getByRole('button', { name: 'Conceder acesso' }).click()
    await expect(page.getByRole('radio', { name: 'Gerar senha para eu informar' })).toBeChecked()
    await page.getByRole('button', { name: 'Gerar senha' }).click()
    const password = (await page.locator('.access-password b').textContent())?.trim() ?? ''
    expect(password).toMatch(/^\d{6}$/)
    await expect(page.locator('.access-result pre')).toContainText(`/${manager.slug}`)

    // Profissional: entra pela página da barbearia com celular + senha.
    const proContext = await browser.newContext({ timezoneId: 'America/Sao_Paulo' })
    const proPage = await proContext.newPage()
    await proPage.goto(`/${manager.slug}`)
    await proPage.getByLabel('Celular').fill(phone.slice(3))
    await proPage.getByLabel('Senha', { exact: true }).fill(password)
    await proPage.getByRole('button', { name: 'Entrar', exact: true }).click()
    await expect(proPage.getByText('Sua agenda')).toBeVisible()
    await expect(proPage.locator('nav').getByText('Config')).toHaveCount(0)

    await proPage.getByRole('button', { name: 'Próximo dia' }).click()
    await expect(proPage.getByText('Cliente do Barbeiro')).toBeVisible()
    await expect(proPage.getByText('Cliente do Outro')).toHaveCount(0)

    // Troca a senha no Perfil e entra de novo com a nova.
    await proPage.locator('header .avatar').click()
    await proPage.getByRole('button', { name: 'Alterar senha' }).click()
    await proPage.getByLabel('Senha atual').fill(password)
    await proPage.getByLabel('Nova senha', { exact: true }).fill('minha-senha-nova')
    await proPage.getByLabel('Confirme a nova senha').fill('minha-senha-nova')
    await proPage.getByRole('button', { name: 'Salvar nova senha' }).click()
    await expect(proPage.getByRole('status')).toHaveText('Senha alterada.')
    await proPage.getByRole('button', { name: 'Sair' }).click()

    await proPage.getByLabel('Celular').fill(phone.slice(3))
    await proPage.getByLabel('Senha', { exact: true }).fill(password)
    await proPage.getByRole('button', { name: 'Entrar', exact: true }).click()
    await expect(proPage.getByRole('alert')).toContainText('inválidos')
    await proPage.getByLabel('Senha', { exact: true }).fill('minha-senha-nova')
    await proPage.getByRole('button', { name: 'Entrar', exact: true }).click()
    await expect(proPage.getByText('Sua agenda')).toBeVisible()
    await proContext.close()
  })
})
