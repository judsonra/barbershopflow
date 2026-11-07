import { afterEach, describe, expect, it } from 'vitest'
import { applyTheme, COLOR_SCHEMES, findScheme, RESERVED_SLUGS, tenantSlugFromPath } from './branding'

describe('tenantSlugFromPath', () => {
  it.each([
    ['/junin', 'junin'],
    ['/junin/', 'junin'],
    ['/Junin-Barber', 'junin-barber'],
    ['/', null],
    ['', null],
    ['/junin/agenda', null],
    ['/sw.js', null],
    ['/-junin', null],
    ['/%E0%A4%A', null],
  ])('%s → %s', (path, expected) => {
    expect(tenantSlugFromPath(path)).toBe(expected)
  })

  it('ignora caminhos reservados do próprio app', () => {
    for (const slug of RESERVED_SLUGS) expect(tenantSlugFromPath(`/${slug}`)).toBeNull()
  })
})

describe('applyTheme', () => {
  afterEach(() => applyTheme(null))

  it('aplica as variáveis da paleta e a cor da barra do navegador', () => {
    applyTheme('vinho')
    const vinho = findScheme('vinho')
    const root = document.documentElement
    expect(root.style.getPropertyValue('--brand-dark')).toBe(vinho.vars.dark)
    expect(root.style.getPropertyValue('--brand-primary')).toBe(vinho.vars.primary)
    expect(root.dataset.colorScheme).toBe('vinho')
    expect(document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')?.content).toBe(vinho.vars.dark)
  })

  it('paleta desconhecida ou vazia volta para o Clássico', () => {
    applyTheme('neon')
    expect(document.documentElement.dataset.colorScheme).toBe('classico')
    applyTheme(undefined)
    expect(document.documentElement.style.getPropertyValue('--brand-dark')).toBe('#111827')
  })

  it('tem as mesmas 8 paletas do backend', () => {
    expect(COLOR_SCHEMES.map(s => s.id)).toEqual(['classico', 'marinho', 'vinho', 'floresta', 'grafite', 'terracota', 'oceano', 'lavanda'])
  })
})
