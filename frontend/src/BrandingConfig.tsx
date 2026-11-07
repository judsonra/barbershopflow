import { ChangeEvent, useEffect, useRef, useState } from 'react'
import { api, ApiError } from './api'
import { applyTheme, COLOR_SCHEMES, DEFAULT_SCHEME, tenantPageUrl } from './branding'
import type { Tenant } from './types'

const MAX_LOGO_BYTES = 1024 * 1024
const LOGO_TYPES = ['image/png', 'image/jpeg', 'image/webp']

function message(err: unknown) {
  return err instanceof ApiError || err instanceof Error ? err.message : 'Não foi possível concluir a operação.'
}

// Config > Identidade visual: the manager picks one of the ready-made
// palettes and uploads the logo shown on http://domain/<slug> and in the
// app header for everyone in the barbershop.
export function BrandingConfig({ tenant, onSaved, onBack }: { tenant: Tenant; onSaved: (tenant: Tenant) => void; onBack: () => void }) {
  const current = tenant.color_scheme ?? DEFAULT_SCHEME
  const [scheme, setScheme] = useState(current)
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState('')
  const [busy, setBusy] = useState<'' | 'colors' | 'upload' | 'remove'>('')
  const [error, setError] = useState('')
  const [info, setInfo] = useState('')
  const [copied, setCopied] = useState(false)
  const pageUrl = tenantPageUrl(tenant.slug)

  // Live preview while choosing; leaving without saving restores the saved one.
  const saved = useRef(current)
  saved.current = current
  useEffect(() => { applyTheme(scheme) }, [scheme])
  useEffect(() => () => applyTheme(saved.current), [])

  useEffect(() => {
    if (!file) { setPreview(''); return }
    const url = URL.createObjectURL(file)
    setPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [file])

  function pickFile(event: ChangeEvent<HTMLInputElement>) {
    setError(''); setInfo('')
    const picked = event.target.files?.[0] ?? null
    event.target.value = ''
    if (!picked) return
    if (!LOGO_TYPES.includes(picked.type)) { setError('Formato inválido: envie PNG, JPG ou WebP.'); return }
    if (picked.size > MAX_LOGO_BYTES) { setError('A logo deve ter no máximo 1 MB.'); return }
    setFile(picked)
  }

  async function saveColors() {
    setBusy('colors'); setError(''); setInfo('')
    try { onSaved(await api.updateBranding(scheme)); setInfo('Cores salvas.') }
    catch (err) { setError(message(err)) }
    finally { setBusy('') }
  }

  async function uploadLogo() {
    if (!file) return
    setBusy('upload'); setError(''); setInfo('')
    try { onSaved(await api.uploadLogo(file)); setFile(null); setInfo('Logo enviada.') }
    catch (err) { setError(message(err)) }
    finally { setBusy('') }
  }

  async function removeLogo() {
    setBusy('remove'); setError(''); setInfo('')
    try { onSaved(await api.deleteLogo()); setInfo('Logo removida.') }
    catch (err) { setError(message(err)) }
    finally { setBusy('') }
  }

  async function copyLink() {
    try { await navigator.clipboard.writeText(pageUrl); setCopied(true); setTimeout(() => setCopied(false), 2000) }
    catch { setError('Não foi possível copiar. Selecione o endereço e copie manualmente.') }
  }

  return <section className="form-page branding-config"><span className="eyebrow">CONFIGURAÇÕES</span><h2>Identidade visual</h2>
    {error && <div className="alert" role="alert">{error}</div>}
    {info && <p role="status">{info}</p>}

    <div className="branding-block">
      <h3>Link da sua página</h3>
      <p className="hint">Envie para seus clientes: a página abre com a sua logo, nome e cores.</p>
      <div className="page-link">
        <a href={pageUrl} target="_blank" rel="noreferrer">{pageUrl}</a>
        <button type="button" onClick={copyLink}>{copied ? 'Copiado!' : 'Copiar'}</button>
      </div>
    </div>

    <fieldset className="branding-block">
      <legend><h3>Esquema de cores</h3></legend>
      <div className="scheme-grid">
        {COLOR_SCHEMES.map(item => <label key={item.id} className={`scheme-option${scheme === item.id ? ' selected' : ''}`}>
          <input type="radio" name="color-scheme" value={item.id} checked={scheme === item.id} onChange={() => setScheme(item.id)} />
          <span className="scheme-preview" aria-hidden="true" style={{ background: item.vars.surface }}>
            <span style={{ background: item.vars.dark }}><i style={{ background: item.vars.accent }} /></span>
            <b style={{ background: item.vars.primary }} />
          </span>
          <span>{item.name}</span>
        </label>)}
      </div>
      <button type="button" className="primary" disabled={busy !== '' || scheme === current} onClick={saveColors}>
        {busy === 'colors' ? 'Salvando…' : 'Salvar cores'}
      </button>
    </fieldset>

    <div className="branding-block">
      <h3>Logo</h3>
      <p className="hint">PNG, JPG ou WebP de até 1 MB. Recomendado: 512 × 512 px, com fundo transparente.</p>
      <div className="logo-row">
        <div className="logo-frame">
          {preview ? <img src={preview} alt="Prévia da nova logo" />
            : tenant.logo_url ? <img src={tenant.logo_url} alt={`Logo da ${tenant.name}`} />
            : <span>Sem logo</span>}
        </div>
        <div className="logo-actions">
          <label className="file-button">Escolher imagem
            <input type="file" accept="image/png,image/jpeg,image/webp" onChange={pickFile} />
          </label>
          {file && <button type="button" className="primary" disabled={busy !== ''} onClick={uploadLogo}>{busy === 'upload' ? 'Enviando…' : 'Enviar logo'}</button>}
          {file && <button type="button" className="link-button" onClick={() => setFile(null)}>Cancelar</button>}
          {!file && tenant.logo_url && <button type="button" className="link-button" disabled={busy !== ''} onClick={removeLogo}>{busy === 'remove' ? 'Removendo…' : 'Remover logo'}</button>}
        </div>
      </div>
    </div>

    <button type="button" className="link-button" onClick={onBack}>Voltar</button>
  </section>
}
