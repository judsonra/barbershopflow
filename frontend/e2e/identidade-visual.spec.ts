import { test, expect, type Page } from '@playwright/test'
import zlib from 'node:zlib'
import { signUpTenant, unique } from './fixtures'

// Builds a real 64x64 PNG in memory, so the backend's content sniffing sees
// a genuine image without the repo needing a binary fixture.
function makePng(rgba: [number, number, number, number]) {
  const crcTable = Array.from({ length: 256 }, (_, n) => {
    let c = n
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    return c >>> 0
  })
  const crc32 = (buf: Buffer) => {
    let c = 0xffffffff
    for (const byte of buf) c = crcTable[(c ^ byte) & 0xff] ^ (c >>> 8)
    return (c ^ 0xffffffff) >>> 0
  }
  const chunk = (type: string, data: Buffer) => {
    const len = Buffer.alloc(4); len.writeUInt32BE(data.length)
    const body = Buffer.concat([Buffer.from(type), data])
    const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(body))
    return Buffer.concat([len, body, crc])
  }
  const size = 64
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(size, 0); ihdr.writeUInt32BE(size, 4); ihdr[8] = 8; ihdr[9] = 6
  const row = Buffer.concat([Buffer.from([0]), Buffer.from(Array(size).fill(rgba).flat())])
  const raw = Buffer.concat(Array(size).fill(row))
  return Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw)), chunk('IEND', Buffer.alloc(0))])
}

const cssVar = (page: Page, name: string) =>
  page.evaluate(n => getComputedStyle(document.documentElement).getPropertyValue(n).trim(), name)

const VINHO_DARK = '#2a0f17'
const CLASSICO_DARK = '#111827'

test.describe('Identidade visual da barbearia', () => {
  test('gestor escolhe cores e logo, e a página /<slug> abre com a marca', async ({ page, browser, request }) => {
    const tenantName = unique('Marca')
    const manager = await signUpTenant(request, tenantName)

    // Gestor configura em Config > Identidade visual.
    await page.goto('/')
    await page.getByLabel('E-mail').fill(manager.email)
    await page.getByLabel('Senha', { exact: true }).fill(manager.password)
    await page.getByRole('button', { name: 'Entrar', exact: true }).click()
    await expect(page.getByText('Sua agenda')).toBeVisible()
    expect(await cssVar(page, '--brand-dark')).toBe(CLASSICO_DARK)

    await page.locator('nav').getByText('Config').click()
    await page.getByText('Identidade visual').click()
    await expect(page.getByRole('link', { name: `http://localhost:5173/${manager.slug}` })).toBeVisible()

    await page.getByRole('radio', { name: 'Vinho' }).check({ force: true })
    await page.getByRole('button', { name: 'Salvar cores' }).click()
    await expect(page.getByRole('status')).toHaveText('Cores salvas.')

    await page.locator('input[type="file"]').setInputFiles({ name: 'logo.png', mimeType: 'image/png', buffer: makePng([140, 29, 64, 255]) })
    await page.getByRole('button', { name: 'Enviar logo' }).click()
    await expect(page.getByRole('status')).toHaveText('Logo enviada.')

    // Dentro do app, o tema e a logo valem para a barbearia toda.
    await page.getByRole('button', { name: 'Voltar' }).click()
    expect(await cssVar(page, '--brand-dark')).toBe(VINHO_DARK)
    await expect(page.locator('header').getByRole('img', { name: `Logo da ${tenantName}` })).toBeVisible()

    // Visitante anônimo em /<slug>: login com a marca, sem cadastro de barbearia.
    const visitor = await browser.newContext()
    const anon = await visitor.newPage()
    await anon.goto(`/${manager.slug}`)
    await expect(anon.getByRole('heading', { name: tenantName })).toBeVisible()
    const logo = anon.getByRole('img', { name: `Logo da ${tenantName}` })
    await expect(logo).toBeVisible()
    expect(await logo.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBe(64)
    expect(await cssVar(anon, '--brand-dark')).toBe(VINHO_DARK)
    await expect(anon.getByLabel('Celular')).toBeVisible()
    await anon.getByRole('button', { name: 'Sou da equipe e entro com e-mail' }).click()
    await expect(anon.getByRole('button', { name: 'Cadastrar minha barbearia' })).toHaveCount(0)

    // E o gestor entra pela própria página e continua com a marca.
    await anon.getByLabel('E-mail').fill(manager.email)
    await anon.getByLabel('Senha', { exact: true }).fill(manager.password)
    await anon.getByRole('button', { name: 'Entrar', exact: true }).click()
    await expect(anon.getByText('Sua agenda')).toBeVisible()
    expect(await cssVar(anon, '--brand-dark')).toBe(VINHO_DARK)
    await visitor.close()
  })

  test('slug inexistente cai na tela padrão com aviso', async ({ page }) => {
    await page.goto(`/${unique('nao-existe')}`)
    await expect(page.getByRole('alert')).toContainText('Barbearia não encontrada')
    await expect(page.getByRole('heading', { name: 'Entrar' })).toBeVisible()
    await expect(page).toHaveURL('http://localhost:5173/')
    expect(await cssVar(page, '--brand-dark')).toBe(CLASSICO_DARK)
  })
})
