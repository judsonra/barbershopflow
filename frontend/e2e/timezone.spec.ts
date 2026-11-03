import { test, expect } from '@playwright/test'
import { signUpTenant, loginViaUI } from './fixtures'

// The browser zone is fixed so the form's local time always becomes the same
// UTC instant, whatever zone this test runs in.
test.use({ timezoneId: 'America/Sao_Paulo' })

function nextMondayDate() {
  const day = new Date()
  day.setDate(day.getDate() + ((8 - day.getDay()) % 7 || 7))
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${day.getFullYear()}-${pad(day.getMonth() + 1)}-${pad(day.getDate())}`
}

test.describe('Fuso horário da barbearia', () => {
  test('valida a jornada no horário local e permite trocar o fuso', async ({ page, request }) => {
    const manager = await signUpTenant(request)
    const headers = { Authorization: `Bearer ${manager.accessToken}` }
    const post = async (path: string, data: unknown) => {
      const response = await request.post(`/api/v1${path}`, { headers, data })
      expect(response.ok(), await response.text()).toBeTruthy()
      return response.json()
    }
    await post('/services', { name: 'Corte Fuso', duration_minutes: 30, price_cents: 5000 })
    const professional = await post('/professionals', { name: 'Barbeiro Fuso' })
    await post('/customers', { name: 'Cliente Fuso', phone: '+5511988887777' })
    const schedule = await request.put(`/api/v1/professionals/${professional.id}/schedule`, {
      headers, data: { entries: [{ weekday: 1, start_minute: 540, end_minute: 1080 }] },
    })
    expect(schedule.ok(), await schedule.text()).toBeTruthy()

    await loginViaUI(page, manager.email, manager.password)
    await expect(page.getByText('Sua agenda')).toBeVisible()

    const book = async (time: string) => {
      // getByLabel('Cliente') also matches the "Enviar acesso ao app" checkbox.
      await page.getByRole('combobox', { name: /^Cliente/ }).selectOption({ label: 'Cliente Fuso' })
      await page.getByLabel('Serviço').selectOption({ label: 'Corte Fuso · R$ 50,00' })
      await page.getByLabel('Profissional').selectOption({ label: 'Barbeiro Fuso' })
      await page.getByLabel('Data e hora').fill(`${nextMondayDate()}T${time}`)
      await page.getByRole('button', { name: 'Confirmar agendamento' }).click()
    }

    await page.locator('nav button', { hasText: 'Novo' }).click()
    await book('06:00')
    await expect(page.locator('.alert')).toBeVisible()

    await book('16:00')
    await expect(page.getByText('Sua agenda')).toBeVisible()

    await page.getByRole('button', { name: 'Config' }).click()
    await page.getByText('Nome, slug e dados da conta').click()
    await expect(page.getByLabel('Fuso horário')).toHaveValue('America/Sao_Paulo')
    await page.getByLabel('Fuso horário').selectOption('America/Manaus')
    await page.getByRole('button', { name: 'Salvar' }).click()
    await expect(page.getByText('Dados salvos.')).toBeVisible()

    await page.reload()
    await page.getByRole('button', { name: 'Config' }).click()
    await page.getByText('Nome, slug e dados da conta').click()
    await expect(page.getByLabel('Fuso horário')).toHaveValue('America/Manaus')
  })
})
