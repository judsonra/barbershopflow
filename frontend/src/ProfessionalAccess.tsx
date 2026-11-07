import { FormEvent, useState } from 'react'
import { api, ApiError } from './api'
import { fromE164BR, isValidEmail, maskPhone, toE164BR } from './validation'
import type { AccessChannel, AccessGrant, Professional } from './types'

const CHANNELS: { id: AccessChannel; label: string; hint: string }[] = [
  { id: 'manual', label: 'Gerar senha para eu informar', hint: 'Nada é enviado: a senha aparece aqui para você passar ao profissional pessoalmente.' },
  { id: 'whatsapp', label: 'Enviar por WhatsApp', hint: 'Envia a senha por WhatsApp para o celular.' },
  { id: 'sms', label: 'Enviar por SMS', hint: 'Envia a senha por SMS para o celular.' },
  { id: 'email', label: 'Enviar por e-mail', hint: 'Envia a senha e o endereço de acesso por e-mail.' },
]

function message(err: unknown) {
  return err instanceof ApiError || err instanceof Error ? err.message : 'Não foi possível concluir a operação.'
}

// Config > Profissionais > (profissional) > Conceder acesso: gives a
// professional their own login to see their agenda. Without SMS/WhatsApp
// or e-mail configured, "Gerar senha para eu informar" is the way in.
export function ProfessionalAccess({ professional, onClose }: { professional: Professional; onClose: () => void }) {
  const [channel, setChannel] = useState<AccessChannel>('manual')
  const [phone, setPhone] = useState(professional.phone ? fromE164BR(professional.phone) : '')
  const [email, setEmail] = useState(professional.email ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<AccessGrant | null>(null)
  const [copied, setCopied] = useState(false)

  const needsPhone = channel === 'whatsapp' || channel === 'sms'
  const needsEmail = channel === 'email'

  async function submit(event: FormEvent) {
    event.preventDefault(); setError(''); setResult(null); setCopied(false)
    if (needsEmail && !isValidEmail(email)) { setError('Informe um e-mail válido.'); return }
    if (channel === 'manual' && !phone.trim() && !email.trim()) { setError('Informe o celular ou o e-mail com que o profissional vai entrar.'); return }
    if (email.trim() && !isValidEmail(email)) { setError('Informe um e-mail válido.'); return }
    setSaving(true)
    try {
      setResult(await api.grantProfessionalAccess(professional.id, {
        channel, phone: phone.trim() ? toE164BR(phone) : undefined, email: email.trim() || undefined,
      }))
    }
    catch (err) { setError(message(err)) }
    finally { setSaving(false) }
  }

  const loginLabel = result?.login?.startsWith('+') ? fromE164BR(result.login) : result?.login
  const instructions = result?.password
    ? `Olá, ${professional.name}! Seu acesso ao app da barbearia:\n` +
      `Endereço: ${result.login_url}\n` +
      `${result.login?.startsWith('+') ? 'Celular' : 'E-mail'}: ${loginLabel}\n` +
      `Senha: ${result.password}\n` +
      'Depois de entrar, troque a senha em Perfil > Alterar senha.'
    : ''

  async function copy() {
    try { await navigator.clipboard.writeText(instructions); setCopied(true) }
    catch { setError('Não foi possível copiar. Selecione o texto e copie manualmente.') }
  }

  return <form onSubmit={submit} className="quick access-form">
    <p>Dar acesso ao app para <b>{professional.name}</b> ver a própria agenda:</p>
    {error && <div className="alert" role="alert">{error}</div>}
    <fieldset className="access-channels">
      <legend>Como entregar a senha</legend>
      {CHANNELS.map(item => <label key={item.id} className="checkbox">
        <input type="radio" name="access-channel" value={item.id} checked={channel === item.id} onChange={() => { setChannel(item.id); setResult(null) }} />
        {item.label}
      </label>)}
      <small>{CHANNELS.find(item => item.id === channel)?.hint}</small>
    </fieldset>
    <label>Celular{needsPhone ? '' : ' (para entrar com celular)'}
      <input type="tel" placeholder="(11) 99999-0000" required={needsPhone} value={phone} onChange={e => setPhone(maskPhone(e.target.value))} maxLength={16} />
    </label>
    <label>E-mail{needsEmail ? '' : ' (opcional, para entrar com e-mail)'}
      <input type="email" required={needsEmail} value={email} onChange={e => setEmail(e.target.value)} />
    </label>
    <button disabled={saving}>{saving ? 'Gerando…' : channel === 'manual' ? 'Gerar senha' : 'Enviar senha'}</button>

    {result && !result.password && <p role="status">{result.message}</p>}
    {result?.password && <div className="access-result" role="status">
      <p>{result.message}</p>
      <div className="access-password"><small>Senha temporária</small><b>{result.password}</b></div>
      <pre>{instructions}</pre>
      <button type="button" onClick={copy}>{copied ? 'Copiado!' : 'Copiar instruções'}</button>
    </div>}
    <button type="button" onClick={onClose}>Fechar</button>
  </form>
}
