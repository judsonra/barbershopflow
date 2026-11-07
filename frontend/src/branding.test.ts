import { afterEach, describe, expect, it, vi } from 'vitest'
import { applyTheme, COLOR_SCHEMES, findScheme, forgetSessionTheme, rememberSessionTheme, rememberSlugTheme, RESERVED_SLUGS, restoreTheme, tenantSlugFromPath } from './branding'

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

describe('restoreTheme (paleta lembrada antes do primeiro render)', () => {
  afterEach(() => { localStorage.clear(); applyTheme(null); vi.restoreAllMocks() })
  const current = () => document.documentElement.dataset.colorScheme

  it('logado: aplica a paleta da barbearia em que entrou', () => {
    rememberSessionTheme('oceano')
    expect(restoreTheme('/', true)).toBe('oceano')
    expect(current()).toBe('oceano')
  })

  it('sem sessão: aplica só a paleta lembrada da página /<slug>', () => {
    rememberSessionTheme('oceano')
    rememberSlugTheme('junin', 'vinho')
    expect(restoreTheme('/junin', false)).toBe('vinho')
    expect(current()).toBe('vinho')
    applyTheme(null)
    expect(restoreTheme('/', false)).toBeNull()
    expect(restoreTheme('/outra', false)).toBeNull()
    expect(current()).toBe('classico')
  })

  it('logout esquece a paleta da sessão', () => {
    rememberSessionTheme('lavanda')
    forgetSessionTheme()
    expect(restoreTheme('/', true)).toBeNull()
  })

  it('sem localStorage disponível, não quebra e não aplica nada', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('bloqueado') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('bloqueado') })
    expect(() => rememberSessionTheme('vinho')).not.toThrow()
    expect(restoreTheme('/', true)).toBeNull()
  })
})
