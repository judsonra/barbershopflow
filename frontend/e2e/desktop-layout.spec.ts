import { test, expect, type Page } from '@playwright/test'
import { signUpTenant, loginViaUI, SEED_SUPERADMIN } from './fixtures'

// The desktop layout (>= 1024px) is pure CSS, so jsdom can't see it: these
// checks measure real boxes in a browser. Below 1024px the app must keep the
// original phone-style column with the menu fixed at the bottom.
async function navBox(page: Page) {
  const box = await page.locator('nav').boundingBox()
  expect(box).not.toBeNull()
  return box!
}

test.describe('Layout desktop', () => {
  test('superadmin no desktop: menu lateral e barbearias em grade', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    await loginViaUI(page, SEED_SUPERADMIN.email, SEED_SUPERADMIN.password)
    await expect(page.getByText('SUPERADMIN')).toBeVisible()

    const nav = await navBox(page)
    expect(nav.x).toBe(0)
    expect(nav.height).toBeGreaterThanOrEqual(900 - 1)
    expect(nav.width).toBeLessThan(300)

    const shell = await page.locator('.app-shell').boundingBox()
    expect(shell!.width).toBeGreaterThan(1300)

    const items = page.locator('.tenant-list li')
    await expect(items.nth(1)).toBeVisible()
    const [first, second] = [await items.nth(0).boundingBox(), await items.nth(1).boundingBox()]
    expect(second!.y).toBe(first!.y) // lado a lado, não empilhados

    // A página precisa rolar quando a lista é maior que a tela.
    const overflowY = await page.evaluate(() => getComputedStyle(document.documentElement).overflowY)
    expect(overflowY).toBe('scroll')
  })

  test('gestor no desktop: menu lateral na agenda', async ({ page, request }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    const manager = await signUpTenant(request)
    await loginViaUI(page, manager.email, manager.password)
    await expect(page.getByText('Sua agenda')).toBeVisible()

    const nav = await navBox(page)
    expect(nav.x).toBe(0)
    expect(nav.height).toBeGreaterThan(nav.width)
  })

  test('celular continua com a coluna única e o menu embaixo', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await loginViaUI(page, SEED_SUPERADMIN.email, SEED_SUPERADMIN.password)
    await expect(page.getByText('SUPERADMIN')).toBeVisible()

    const nav = await navBox(page)
    expect(nav.width).toBe(390)
    expect(nav.y + nav.height).toBeCloseTo(844, 0)

    const items = page.locator('.tenant-list li')
    await expect(items.nth(1)).toBeVisible()
    const [first, second] = [await items.nth(0).boundingBox(), await items.nth(1).boundingBox()]
    expect(second!.y).toBeGreaterThan(first!.y)
  })
})
