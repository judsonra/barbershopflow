// Identidade visual por barbearia: paletas prontas (ids iguais aos de
// domain.ColorSchemes no backend) e a leitura do slug em http://domínio/<slug>.

export type ColorVars = {
  dark: string; dark2: string; accent: string; primary: string; primaryInk: string
  link: string; time: string; soft: string; soft2: string; pageBg: string; surface: string
}
export type ColorScheme = { id: string; name: string; vars: ColorVars }

export const DEFAULT_SCHEME = 'classico'

// Todas têm cabeçalho escuro (o texto do cabeçalho é sempre branco) e cor
// principal com contraste suficiente para texto branco nos botões.
export const COLOR_SCHEMES: ColorScheme[] = [
  { id: 'classico', name: 'Clássico', vars: { dark: '#111827', dark2: '#1f2937', accent: '#d6a85f', primary: '#b68034', primaryInk: '#ffffff', link: '#906521', time: '#b07d31', soft: '#e9dfcf', soft2: '#f2eee6', pageBg: '#e8e5df', surface: '#f7f4ee' } },
  { id: 'marinho', name: 'Azul marinho', vars: { dark: '#0f1f3d', dark2: '#1c3160', accent: '#8fb4ff', primary: '#2f5bd3', primaryInk: '#ffffff', link: '#24479e', time: '#2f5bd3', soft: '#dbe4f7', soft2: '#eaf0fb', pageBg: '#e3e8f0', surface: '#f5f7fb' } },
  { id: 'vinho', name: 'Vinho', vars: { dark: '#2a0f17', dark2: '#44182a', accent: '#e3a4b4', primary: '#8c1d40', primaryInk: '#ffffff', link: '#7a1838', time: '#8c1d40', soft: '#f0dbe1', soft2: '#f7ecef', pageBg: '#ebe2e4', surface: '#faf5f6' } },
  { id: 'floresta', name: 'Verde floresta', vars: { dark: '#0f2419', dark2: '#1b3a2a', accent: '#9fd4a8', primary: '#2f7a4b', primaryInk: '#ffffff', link: '#24603a', time: '#2f7a4b', soft: '#dcebdf', soft2: '#edf5ef', pageBg: '#e2e9e3', surface: '#f5f9f6' } },
  { id: 'grafite', name: 'Grafite', vars: { dark: '#1a1a1a', dark2: '#2c2c2c', accent: '#e5e5e5', primary: '#3f3f46', primaryInk: '#ffffff', link: '#27272a', time: '#52525b', soft: '#e4e4e7', soft2: '#f0f0f2', pageBg: '#e7e7e9', surface: '#f8f8f9' } },
  { id: 'terracota', name: 'Terracota', vars: { dark: '#2b1a12', dark2: '#45291c', accent: '#f0b48a', primary: '#b04d25', primaryInk: '#ffffff', link: '#9c4320', time: '#b04d25', soft: '#f3ddd0', soft2: '#f9ece4', pageBg: '#ece4de', surface: '#fbf6f2' } },
  { id: 'oceano', name: 'Oceano', vars: { dark: '#062a33', dark2: '#0d3f4b', accent: '#7fd6e0', primary: '#0e7c86', primaryInk: '#ffffff', link: '#0b5f67', time: '#0e7c86', soft: '#d4ecee', soft2: '#e8f5f6', pageBg: '#dfe9ea', surface: '#f4fafa' } },
  { id: 'lavanda', name: 'Lavanda', vars: { dark: '#221a33', dark2: '#362a52', accent: '#c8b5f0', primary: '#6b4fbb', primaryInk: '#ffffff', link: '#563f99', time: '#6b4fbb', soft: '#e6dff5', soft2: '#f1ecfa', pageBg: '#e7e3ee', surface: '#f8f6fc' } },
]

const CSS_VARS: Record<keyof ColorVars, string> = {
  dark: '--brand-dark', dark2: '--brand-dark-2', accent: '--brand-accent', primary: '--brand-primary', primaryInk: '--brand-primary-ink',
  link: '--brand-link', time: '--brand-time', soft: '--brand-soft', soft2: '--brand-soft-2', pageBg: '--page-bg', surface: '--surface',
}

export function findScheme(id?: string | null): ColorScheme {
  return COLOR_SCHEMES.find(s => s.id === id) ?? COLOR_SCHEMES[0]
}

// Aplica a paleta nas variáveis CSS da página e na cor da barra do navegador
// (meta theme-color). null/desconhecido volta para o Clássico.
export function applyTheme(id?: string | null) {
  const scheme = findScheme(id)
  const root = document.documentElement
  for (const [key, cssVar] of Object.entries(CSS_VARS) as [keyof ColorVars, string][]) {
    root.style.setProperty(cssVar, scheme.vars[key])
  }
  root.dataset.colorScheme = scheme.id
  let meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')
  if (!meta) {
    meta = document.createElement('meta')
    meta.name = 'theme-color'
    document.head.appendChild(meta)
  }
  meta.content = scheme.vars.dark
}

// Mesma lista de domain.ReservedSlugs: caminhos que pertencem ao próprio app.
export const RESERVED_SLUGS = ['api', 'health', 'assets', 'admin', 'login', 'static', 'app', 'www']
const SLUG_PATTERN = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/

// "/junin" ou "/junin/" → "junin". Qualquer outro formato (raiz, subpasta,
// arquivo, slug reservado) → null, e o app abre a tela padrão.
export function tenantSlugFromPath(pathname: string): string | null {
  const match = /^\/([^/]+)\/?$/.exec(pathname)
  if (!match) return null
  let slug: string
  try { slug = decodeURIComponent(match[1]).toLowerCase() } catch { return null }
  if (!SLUG_PATTERN.test(slug) || RESERVED_SLUGS.includes(slug)) return null
  return slug
}

export function tenantPageUrl(slug: string) {
  return `${window.location.origin}/${slug}`
}
