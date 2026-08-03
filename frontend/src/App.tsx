import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { api, ApiError } from './api'
import { downloadICS, googleCalendarUrl } from './calendar'
import { isValidEmail, maskPhone, toE164BR } from './validation'
import type { Appointment, Customer, Professional, Service, Tenant, TimeOff, User } from './types'

function errorMessage(err: unknown) {
  return err instanceof Error ? err.message : 'Erro inesperado'
}

const roleLabel: Record<string, string> = { manager: 'Gestor', professional: 'Profissional', client: 'Cliente', superadmin: 'Superadmin' }
const money = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })
const dateTime = new Intl.DateTimeFormat('pt-BR', { weekday: 'short', day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit', hour12: false })
const statusLabel = { scheduled: 'Agendado', confirmed: 'Confirmado', completed: 'Concluído', cancelled: 'Cancelado' }

function dayBounds(offset = 0) {
  const from = new Date(); from.setDate(from.getDate() + offset); from.setHours(0, 0, 0, 0)
  const to = new Date(from); to.setDate(to.getDate() + 1)
  return { from: from.toISOString(), to: to.toISOString() }
}

export default function App() {
  const [user, setUser] = useState<User | null>(null)
  const [checkingSession, setCheckingSession] = useState(true)
  const [oauthError, setOauthError] = useState('')

  useEffect(() => {
    api.onSessionExpired(() => setUser(null))
    const { error } = api.consumeOAuthRedirect()
    if (error) setOauthError(error)
    if (!api.isAuthenticated()) { setCheckingSession(false); return }
    api.me().then(setUser).catch(() => api.logout()).finally(() => setCheckingSession(false))
  }, [])

  if (checkingSession) return <div className="empty">Carregando…</div>
  if (!user) return <Login onLogin={setUser} initialError={oauthError} />
  if (user.role === 'superadmin') return <AdminPanel user={user} onImpersonate={setUser} onLogout={() => { api.logout(); setUser(null) }} />
  return <AgendaApp user={user} onLogout={() => { api.logout(); setUser(null) }} />
}

// A superadmin doesn't operate inside any single tenant — it only lists
// barbershops and "becomes" one's manager to actually do anything (see
// docs/api.md). Logging back out returns to this same login; to switch
// back to the admin view, log in again with the superadmin account.
function AdminPanel({ user, onImpersonate, onLogout }: { user: User; onImpersonate: (user: User) => void; onLogout: () => void }) {
  const [view, setView] = useState<'tenants' | 'profile'>('tenants')
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true); setError('')
    try { setTenants(await api.listTenantsAdmin()) }
    catch (err) { setError(errorMessage(err)) }
    finally { setLoading(false) }
  }, [])

  useEffect(() => { void load() }, [load])

  async function access(tenantId: string) {
    setError('')
    try { onImpersonate(await api.impersonateTenant(tenantId)) }
    catch (err) { setError(errorMessage(err)) }
  }

  return <div className="app-shell">
    <header>
      <div><span className="eyebrow">SUPERADMIN</span><h1>Barbearias</h1></div>
      <button className="avatar" title={`${user.name} · Perfil`} onClick={() => setView('profile')}>{user.name.slice(0, 2).toUpperCase()}</button>
    </header>
    <main>
      {view === 'profile' ? <Profile user={user} onLogout={onLogout} onBack={() => setView('tenants')} /> : <>
        {error && <div className="alert" role="alert">{error}<button onClick={() => setError('')}>×</button></div>}
        {loading ? <div className="empty">Carregando…</div> :
          tenants.length === 0 ? <div className="empty"><span>🏠</span><h2>Nenhuma barbearia</h2></div> :
          <ul className="tenant-list">{tenants.map(t => <li key={t.id}>
            <div><b>{t.name}</b><small>{t.slug}{!t.active ? ' · inativa' : ''}</small></div>
            <button onClick={() => void access(t.id)}>Acessar como gestor</button>
          </li>)}</ul>}
      </>}
    </main>
  </div>
}

// Shared by AgendaApp and AdminPanel: shows the signed-in account's own
// data and is the only place "Sair" lives, so clicking the avatar never
// logs anyone out by surprise anymore.
function Profile({ user, onLogout, onBack }: { user: User; onLogout: () => void; onBack: () => void }) {
  return <section className="form-page">
    <span className="eyebrow">PERFIL</span><h2>{user.name}</h2>
    <div className="profile-info">
      <div><small>Papel</small><b>{roleLabel[user.role] ?? user.role}</b></div>
      {user.email && <div><small>E-mail</small><b>{user.email}</b></div>}
      {user.phone && <div><small>Celular</small><b>{user.phone}</b></div>}
      <div><small>Status</small><b>{user.active ? 'Ativo' : 'Inativo'}</b></div>
    </div>
    <button className="primary danger" onClick={onLogout}>Sair</button>
    <button type="button" className="link-button" onClick={onBack}>Voltar</button>
  </section>
}

type LoginMode = 'email' | 'phone' | 'recover' | 'signup'

function slugify(value: string) {
  return value.toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
}

function Login({ onLogin, initialError }: { onLogin: (user: User) => void; initialError?: string }) {
  const [mode, setMode] = useState<LoginMode>('email')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState(initialError ?? '')
  const [info, setInfo] = useState('')
  const [loading, setLoading] = useState(false)
  const [signup, setSignup] = useState({ tenantName: '', managerName: '', email: '', password: '' })

  async function submitEmail(event: FormEvent) {
    event.preventDefault(); setLoading(true); setError('')
    try { onLogin(await api.login(email, password)) }
    catch (err) { setError(errorMessage(err)) }
    finally { setLoading(false) }
  }

  async function submitSignup(event: FormEvent) {
    event.preventDefault(); setError('')
    // Some mobile browsers (notably installed PWAs) silently swallow the
    // native minLength validation bubble instead of showing it — the form
    // just sits there with no feedback. Check explicitly so there's always
    // a visible message.
    if (signup.password.length < 8) { setError('A senha deve ter pelo menos 8 caracteres.'); return }
    if (!isValidEmail(signup.email)) { setError('Informe um e-mail válido.'); return }
    setLoading(true)
    try {
      onLogin(await api.createTenant({
        tenant_name: signup.tenantName, slug: slugify(signup.tenantName),
        manager_name: signup.managerName, email: signup.email, password: signup.password
      }))
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setLoading(false) }
  }

  async function submitPhone(event: FormEvent) {
    event.preventDefault(); setLoading(true); setError('')
    try { onLogin(await api.loginByPhone(toE164BR(phone), password)) }
    catch (err) {
      if (err instanceof ApiError && err.code === 'account_locked') { setMode('recover'); setError('') }
      else setError(errorMessage(err))
    }
    finally { setLoading(false) }
  }

  async function submitRecover(event: FormEvent) {
    event.preventDefault(); setLoading(true); setError(''); setInfo('')
    try { setInfo((await api.recoverPhone(toE164BR(phone))).message) }
    catch (err) { setError(errorMessage(err)) }
    finally { setLoading(false) }
  }

  return <div className="app-shell">
    <header>
      <div><span className="eyebrow">BARBERFLOW</span><h1>Entrar</h1></div>
    </header>
    <main>
      <section className="form-page">
        {error && <div className="alert" role="alert">{error}</div>}

        {mode === 'email' && <>
          <form onSubmit={submitEmail}>
            <label>E-mail<input type="email" required autoComplete="username" value={email} onChange={e => setEmail(e.target.value)} /></label>
            <label>Senha<input type="password" required autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} /></label>
            <button className="primary" disabled={loading}>{loading ? 'Entrando…' : 'Entrar'}</button>
          </form>
          <div className="social-login">
            <a className="social-button" href={api.socialLoginUrl('google')}>Entrar com Google</a>
            <a className="social-button" href={api.socialLoginUrl('facebook')}>Entrar com Facebook</a>
          </div>
          <button type="button" className="link-button" onClick={() => { setMode('phone'); setError('') }}>Sou cliente e entro com celular</button>
          <button type="button" className="link-button" onClick={() => { setMode('signup'); setError('') }}>Cadastrar minha barbearia</button>
        </>}

        {mode === 'signup' && <>
          <form onSubmit={submitSignup}>
            <label>Nome da barbearia<input required value={signup.tenantName} onChange={e => setSignup({ ...signup, tenantName: e.target.value })} /></label>
            <label>Seu nome<input required value={signup.managerName} onChange={e => setSignup({ ...signup, managerName: e.target.value })} /></label>
            <label>E-mail<input type="email" required autoComplete="username" value={signup.email} onChange={e => setSignup({ ...signup, email: e.target.value })} /></label>
            <label>Senha<input type="password" required minLength={8} autoComplete="new-password" value={signup.password} onChange={e => setSignup({ ...signup, password: e.target.value })} /></label>
            <button className="primary" disabled={loading}>{loading ? 'Criando…' : 'Criar barbearia'}</button>
          </form>
          <button type="button" className="link-button" onClick={() => { setMode('email'); setError('') }}>Já tenho conta</button>
        </>}

        {mode === 'phone' && <>
          <form onSubmit={submitPhone}>
            <label>Celular<input type="tel" required autoComplete="tel" placeholder="(11) 99999-0000" value={phone} onChange={e => setPhone(maskPhone(e.target.value))} maxLength={16} /></label>
            <label>Senha<input type="password" required autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} /></label>
            <button className="primary" disabled={loading}>{loading ? 'Entrando…' : 'Entrar'}</button>
          </form>
          <button type="button" className="link-button" onClick={() => { setMode('recover'); setError('') }}>Esqueci minha senha</button>
          <button type="button" className="link-button" onClick={() => { setMode('email'); setError('') }}>Entrar com e-mail</button>
        </>}

        {mode === 'recover' && <>
          {info
            ? <p>{info}</p>
            : <form onSubmit={submitRecover}>
                <label>Celular cadastrado<input type="tel" required autoComplete="tel" placeholder="(11) 99999-0000" value={phone} onChange={e => setPhone(maskPhone(e.target.value))} maxLength={16} /></label>
                <button className="primary" disabled={loading}>{loading ? 'Enviando…' : 'Receber nova senha por SMS/WhatsApp'}</button>
              </form>}
          <button type="button" className="link-button" onClick={() => { setMode('phone'); setError(''); setInfo('') }}>Voltar para o login</button>
        </>}
      </section>
    </main>
  </div>
}

function AgendaApp({ user, onLogout }: { user: User; onLogout: () => void }) {
  const isClient = user.role === 'client'
  const [tab, setTab] = useState<'agenda' | 'new' | 'config' | 'hours' | 'profile'>('agenda')
  const [offset, setOffset] = useState(0)
  const [appointments, setAppointments] = useState<Appointment[]>([])
  const [services, setServices] = useState<Service[]>([])
  const [professionals, setProfessionals] = useState<Professional[]>([])
  const [customers, setCustomers] = useState<Customer[]>([])
  const [tenant, setTenant] = useState<Tenant | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const bounds = useMemo(() => dayBounds(offset), [offset])

  const load = useCallback(async () => {
    setLoading(true); setError('')
    try {
      const [agenda, serviceList, professionalList, tenantInfo, customerList] = await Promise.all([
        api.appointments(bounds.from, bounds.to), api.services(), api.professionals(), api.tenant(),
        isClient ? Promise.resolve([]) : api.customers()
      ])
      setAppointments(agenda); setServices(serviceList); setProfessionals(professionalList)
      setTenant(tenantInfo); setCustomers(customerList)
    } catch (err) { setError(errorMessage(err)) }
    finally { setLoading(false) }
  }, [bounds, isClient])

  useEffect(() => { void load() }, [load])

  async function changeStatus(item: Appointment, status: Appointment['status']) {
    try { await api.updateStatus(item.id, status); await load() }
    catch (err) { setError(errorMessage(err)) }
  }

  return <div className="app-shell">
    <header>
      <div><span className="eyebrow">BARBERFLOW</span><h1>Sua agenda</h1></div>
      <button className="avatar" title={`${user.name} · Perfil`} onClick={() => setTab('profile')}>{user.name.slice(0, 2).toUpperCase()}</button>
    </header>

    <main>
      {error && <div className="alert" role="alert">{error}<button onClick={() => setError('')}>×</button></div>}
      {tab === 'agenda' && <>
        <section className="date-nav">
          <button aria-label="Dia anterior" onClick={() => setOffset(v => v - 1)}>‹</button>
          <div><b>{offset === 0 ? 'Hoje' : dateTime.format(new Date(bounds.from)).split(' às')[0]}</b><small>{appointments.length} atendimento(s)</small></div>
          <button aria-label="Próximo dia" onClick={() => setOffset(v => v + 1)}>›</button>
        </section>
        {loading ? <div className="empty">Carregando agenda…</div> :
          appointments.length === 0 ? <div className="empty"><span>✂</span><h2>Agenda livre</h2>
            <p>{isClient ? 'Você ainda não tem horário marcado.' : 'Que tal criar o primeiro horário do dia?'}</p>
          </div> :
          <section className="appointments">{appointments.map(item => <article className={`card ${item.status}`} key={item.id}>
            <time>{new Date(item.starts_at).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit', hour12: false })}</time>
            <div className="card-body">
              <div className="card-title"><h3>{item.customer_name}</h3><span>{statusLabel[item.status]}</span></div>
              <p>{item.service_name} · {item.professional_name}</p>
              <b>{money.format(item.price_cents / 100)}</b>
              {(item.status === 'scheduled' || item.status === 'confirmed') && <div className="actions">
                <a href={googleCalendarUrl(item)} target="_blank" rel="noreferrer">Google Agenda</a>
                <button type="button" onClick={() => downloadICS(item)}>Baixar .ics</button>
              </div>}
              {!isClient && item.status === 'scheduled' && <div className="actions"><button onClick={() => changeStatus(item, 'confirmed')}>Confirmar</button><button onClick={() => changeStatus(item, 'cancelled')}>Cancelar</button></div>}
              {!isClient && item.status === 'confirmed' && <div className="actions"><button onClick={() => changeStatus(item, 'completed')}>Concluir</button><button onClick={() => changeStatus(item, 'cancelled')}>Cancelar</button></div>}
            </div>
          </article>)}</section>}
      </>}
      {tab === 'new' && <NewAppointment user={user} tenant={tenant} services={services} professionals={professionals} customers={customers}
        onDone={async () => { setTab('agenda'); setOffset(0); await load() }} />}
      {tab === 'config' && tenant && <>
        <TenantConfig tenant={tenant} onSaved={t => setTenant(t)} />
        <ServicesManager services={services} onCreated={s => setServices(v => [...v, s])} />
        <ProfessionalsManager professionals={professionals} onCreated={p => setProfessionals(v => [...v, p])} />
      </>}
      {tab === 'hours' && <ScheduleManager user={user} professionals={professionals} />}
      {tab === 'profile' && <Profile user={user} onLogout={onLogout} onBack={() => setTab('agenda')} />}
    </main>

    {tab === 'agenda' && (!isClient || tenant?.self_scheduling_enabled) &&
      <button className="fab" aria-label="Novo agendamento" title="Novo agendamento" onClick={() => setTab('new')}>+</button>}

    <nav>
      <button className={tab === 'agenda' ? 'active' : ''} onClick={() => setTab('agenda')}><span>▦</span>Agenda</button>
      {(!isClient || tenant?.self_scheduling_enabled) &&
        <button className={tab === 'new' ? 'active add' : 'add'} onClick={() => setTab('new')}><span>＋</span>Novo</button>}
      {!isClient &&
        <button className={tab === 'hours' ? 'active' : ''} onClick={() => setTab('hours')}><span>🕘</span>Horários</button>}
      {user.role === 'manager' &&
        <button className={tab === 'config' ? 'active' : ''} onClick={() => setTab('config')}><span>⚙</span>Config</button>}
    </nav>
  </div>
}

function TenantConfig({ tenant, onSaved }: { tenant: Tenant; onSaved: (tenant: Tenant) => void }) {
  const [selfScheduling, setSelfScheduling] = useState(tenant.self_scheduling_enabled)
  const [autoConfirm, setAutoConfirm] = useState(tenant.auto_confirm_appointments)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function save() {
    setSaving(true); setError('')
    try { onSaved(await api.updateTenant({ self_scheduling_enabled: selfScheduling, auto_confirm_appointments: autoConfirm })) }
    catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }

  return <section className="form-page"><span className="eyebrow">CONFIGURAÇÕES</span><h2>{tenant.name}</h2>
    {error && <div className="alert">{error}</div>}
    <form onSubmit={e => { e.preventDefault(); void save() }}>
      <label className="checkbox"><input type="checkbox" checked={selfScheduling} onChange={e => setSelfScheduling(e.target.checked)} /> Permitir que clientes sugiram o próprio horário (autoagendamento)</label>
      <label className="checkbox"><input type="checkbox" checked={autoConfirm} disabled={!selfScheduling} onChange={e => setAutoConfirm(e.target.checked)} /> Confirmar automaticamente o horário sugerido pelo cliente</label>
      <p>{selfScheduling
        ? (autoConfirm ? 'O horário do cliente já entra confirmado e reservado.' : 'O horário do cliente fica reservado como pendente até o profissional confirmar.')
        : 'Só a equipe cria agendamentos.'}</p>
      <button className="primary" disabled={saving}>{saving ? 'Salvando…' : 'Salvar'}</button>
    </form>
  </section>
}

function ServicesManager({ services, onCreated }: { services: Service[]; onCreated: (service: Service) => void }) {
  const [name, setName] = useState(''); const [duration, setDuration] = useState('30'); const [price, setPrice] = useState('')
  const [saving, setSaving] = useState(false); const [error, setError] = useState('')

  async function submit(event: FormEvent) {
    event.preventDefault(); setError('')
    const durationMinutes = Number(duration)
    const priceCents = Math.round(Number(price.replace(',', '.')) * 100)
    if (!Number.isFinite(durationMinutes) || durationMinutes < 5 || durationMinutes > 480) {
      setError('Duração deve ser entre 5 e 480 minutos.'); return
    }
    if (!price.trim() || !Number.isFinite(priceCents) || priceCents < 0) { setError('Informe um preço válido.'); return }
    setSaving(true)
    try {
      onCreated(await api.createService({ name, duration_minutes: durationMinutes, price_cents: priceCents }))
      setName(''); setDuration('30'); setPrice('')
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }

  return <section className="form-page"><h2>Serviços</h2>
    {error && <div className="alert">{error}</div>}
    {services.length === 0 ? <p>Nenhum serviço cadastrado.</p> : <ul className="tenant-list">
      {services.map(s => <li key={s.id}><div><b>{s.name}</b><small>{s.duration_minutes} min · {money.format(s.price_cents / 100)}{!s.active ? ' · inativo' : ''}</small></div></li>)}
    </ul>}
    <form onSubmit={submit} className="quick">
      <input placeholder="Nome" required value={name} onChange={e => setName(e.target.value)} />
      <input type="number" min={5} max={480} placeholder="Duração (minutos)" required value={duration} onChange={e => setDuration(e.target.value)} />
      <input type="text" inputMode="decimal" placeholder="Preço (R$)" required value={price} onChange={e => setPrice(e.target.value)} />
      <button disabled={saving}>{saving ? 'Adicionando…' : 'Adicionar serviço'}</button>
    </form>
  </section>
}

function ProfessionalsManager({ professionals, onCreated }: { professionals: Professional[]; onCreated: (professional: Professional) => void }) {
  const [name, setName] = useState(''); const [phone, setPhone] = useState('')
  const [saving, setSaving] = useState(false); const [error, setError] = useState('')

  async function submit(event: FormEvent) {
    event.preventDefault(); setSaving(true); setError('')
    try {
      onCreated(await api.createProfessional({ name, phone: toE164BR(phone) }))
      setName(''); setPhone('')
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }

  return <section className="form-page"><h2>Profissionais</h2>
    {error && <div className="alert">{error}</div>}
    {professionals.length === 0 ? <p>Nenhum profissional cadastrado.</p> : <ul className="tenant-list">
      {professionals.map(p => <li key={p.id}><div><b>{p.name}</b>{!p.active && <small> · inativo</small>}</div></li>)}
    </ul>}
    <form onSubmit={submit} className="quick">
      <input placeholder="Nome" required value={name} onChange={e => setName(e.target.value)} />
      <input type="tel" placeholder="(11) 99999-0000 (opcional)" value={phone} onChange={e => setPhone(maskPhone(e.target.value))} maxLength={16} />
      <button disabled={saving}>{saving ? 'Adicionando…' : 'Adicionar profissional'}</button>
    </form>
  </section>
}

const WEEKDAYS = ['Domingo', 'Segunda', 'Terça', 'Quarta', 'Quinta', 'Sexta', 'Sábado']

function minutesToTime(minutes: number) {
  return `${Math.floor(minutes / 60).toString().padStart(2, '0')}:${(minutes % 60).toString().padStart(2, '0')}`
}
function timeToMinutes(value: string) {
  const [hours, minutes] = value.split(':').map(Number)
  return hours * 60 + minutes
}

type ScheduleDay = { enabled: boolean; start: string; end: string }
const DEFAULT_SCHEDULE_DAY: ScheduleDay = { enabled: false, start: '09:00', end: '18:00' }

function ScheduleManager({ user, professionals }: { user: User; professionals: Professional[] }) {
  const isManager = user.role === 'manager'
  const [professionalId, setProfessionalId] = useState(isManager ? (professionals[0]?.id ?? '') : (user.professional_id ?? ''))
  const [days, setDays] = useState<ScheduleDay[]>(Array.from({ length: 7 }, () => ({ ...DEFAULT_SCHEDULE_DAY })))
  const [timeOff, setTimeOff] = useState<TimeOff[]>([])
  const [block, setBlock] = useState({ starts_at: '', ends_at: '', reason: '' })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [info, setInfo] = useState('')

  const load = useCallback(async () => {
    if (!professionalId) { setLoading(false); return }
    setLoading(true); setError(''); setInfo('')
    try {
      const [entries, blocks] = await Promise.all([api.getSchedule(professionalId), api.listTimeOff(professionalId)])
      const next = Array.from({ length: 7 }, () => ({ ...DEFAULT_SCHEDULE_DAY }))
      for (const entry of entries) next[entry.weekday] = { enabled: true, start: minutesToTime(entry.start_minute), end: minutesToTime(entry.end_minute) }
      setDays(next); setTimeOff(blocks)
    } catch (err) { setError(errorMessage(err)) }
    finally { setLoading(false) }
  }, [professionalId])

  useEffect(() => { void load() }, [load])

  function updateDay(weekday: number, patch: Partial<ScheduleDay>) {
    setDays(v => v.map((day, i) => i === weekday ? { ...day, ...patch } : day))
  }

  async function saveSchedule() {
    setSaving(true); setError(''); setInfo('')
    try {
      const entries = days
        .map((day, weekday) => ({ weekday, start_minute: timeToMinutes(day.start), end_minute: timeToMinutes(day.end), enabled: day.enabled }))
        .filter(entry => entry.enabled)
        .map(({ weekday, start_minute, end_minute }) => ({ weekday, start_minute, end_minute }))
      await api.setSchedule(professionalId, entries)
      setInfo('Jornada salva.')
    } catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }

  async function addBlock(event: FormEvent) {
    event.preventDefault(); setSaving(true); setError('')
    try {
      const created = await api.createTimeOff(professionalId, {
        starts_at: new Date(block.starts_at).toISOString(), ends_at: new Date(block.ends_at).toISOString(), reason: block.reason
      })
      setTimeOff(v => [...v, created].sort((a, b) => a.starts_at.localeCompare(b.starts_at)))
      setBlock({ starts_at: '', ends_at: '', reason: '' })
    } catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }

  async function removeBlock(id: string) {
    try { await api.deleteTimeOff(professionalId, id); setTimeOff(v => v.filter(b => b.id !== id)) }
    catch (err) { setError(errorMessage(err)) }
  }

  return <section className="form-page"><span className="eyebrow">HORÁRIOS</span><h2>Jornada de trabalho</h2>
    {error && <div className="alert">{error}</div>}
    {isManager && <label>Profissional<select value={professionalId} onChange={e => setProfessionalId(e.target.value)}>
      {professionals.map(p => <option value={p.id} key={p.id}>{p.name}</option>)}
    </select></label>}
    {!professionalId ? <p>Nenhum profissional cadastrado.</p> : loading ? <p>Carregando…</p> : <>
      <p>Dias sem horário marcado ficam de folga fixa. Sem nenhum dia marcado, a agenda fica livre o tempo todo.</p>
      <div className="schedule-grid">
        {WEEKDAYS.map((label, weekday) => <div className="schedule-row" key={weekday}>
          <label className="checkbox"><input type="checkbox" checked={days[weekday].enabled}
            onChange={e => updateDay(weekday, { enabled: e.target.checked })} /> {label}</label>
          {days[weekday].enabled && <div className="schedule-times">
            <input type="time" lang="pt-BR" value={days[weekday].start} onChange={e => updateDay(weekday, { start: e.target.value })} />
            <span>até</span>
            <input type="time" lang="pt-BR" value={days[weekday].end} onChange={e => updateDay(weekday, { end: e.target.value })} />
          </div>}
        </div>)}
      </div>
      {info && <p>{info}</p>}
      <button className="primary" disabled={saving} onClick={() => void saveSchedule()}>{saving ? 'Salvando…' : 'Salvar jornada'}</button>

      <h2>Ausências e bloqueios</h2>
      {timeOff.length === 0 ? <p>Nenhum bloqueio cadastrado.</p> : <ul className="time-off-list">
        {timeOff.map(b => <li key={b.id}>
          <span>{new Date(b.starts_at).toLocaleString('pt-BR', { hour12: false })} até {new Date(b.ends_at).toLocaleString('pt-BR', { hour12: false })}{b.reason ? ` · ${b.reason}` : ''}</span>
          <button type="button" onClick={() => void removeBlock(b.id)}>Remover</button>
        </li>)}
      </ul>}
      <form onSubmit={addBlock} className="quick">
        <label>Início<input type="datetime-local" lang="pt-BR" required value={block.starts_at} onChange={e => setBlock({ ...block, starts_at: e.target.value })} /></label>
        <label>Fim<input type="datetime-local" lang="pt-BR" required value={block.ends_at} onChange={e => setBlock({ ...block, ends_at: e.target.value })} /></label>
        <input placeholder="Motivo (folga, viagem, ausência...)" value={block.reason} onChange={e => setBlock({ ...block, reason: e.target.value })} />
        <button disabled={saving}>Bloquear período</button>
      </form>
    </>}
  </section>
}

function NewAppointment({ user, tenant, services, professionals, customers, onDone }: {
  user: User; tenant: Tenant | null; services: Service[]; professionals: Professional[]; customers: Customer[]; onDone: () => Promise<void>
}) {
  const isClient = user.role === 'client'
  const [form, setForm] = useState({ customer_id: '', professional_id: '', service_id: '', starts_at: '', notes: '' })
  const [name, setName] = useState(''); const [phone, setPhone] = useState(''); const [email, setEmail] = useState(''); const [grantAccess, setGrantAccess] = useState(false)
  const [list, setList] = useState(customers); const [saving, setSaving] = useState(false); const [error, setError] = useState('')
  const [accessMessage, setAccessMessage] = useState('')
  async function addCustomer() {
    if (!name.trim()) return
    setError('')
    if (email.trim() && !isValidEmail(email.trim())) { setError('Informe um e-mail válido ou deixe em branco.'); return }
    try {
      const customer = await api.createCustomer({ name, phone: toE164BR(phone), email })
      setList(v => [...v, customer]); setForm(v => ({ ...v, customer_id: customer.id })); setName(''); setPhone(''); setEmail('')
      if (grantAccess && phone.trim()) {
        setAccessMessage((await api.grantCustomerAccess(customer.id, toE164BR(phone))).message)
      }
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setGrantAccess(false) }
  }
  async function submit(event: FormEvent) {
    event.preventDefault(); setSaving(true); setError('')
    try { await api.createAppointment({ ...form, starts_at: new Date(form.starts_at).toISOString() }); await onDone() }
    catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }
  const pending = isClient && tenant && !tenant.auto_confirm_appointments
  return <section className="form-page"><span className="eyebrow">NOVO HORÁRIO</span><h2>{isClient ? 'Sugerir horário' : 'Agendar atendimento'}</h2>
    {error && <div className="alert">{error}</div>}
    {isClient && <p>{pending ? 'O profissional precisa confirmar antes do horário ficar garantido.' : 'Seu horário é reservado assim que você confirma.'}</p>}
    <form onSubmit={submit}>
      {!isClient && <>
        <label>Cliente<select required value={form.customer_id} onChange={e => setForm({ ...form, customer_id: e.target.value })}><option value="">Selecione</option>{list.map(x => <option value={x.id} key={x.id}>{x.name}</option>)}</select></label>
        <details><summary>Cadastrar cliente rápido</summary><div className="quick">
          <input placeholder="Nome" value={name} onChange={e => setName(e.target.value)} />
          <input type="tel" placeholder="(11) 99999-0000" value={phone} onChange={e => setPhone(maskPhone(e.target.value))} maxLength={16} />
          <input type="email" placeholder="E-mail (opcional)" value={email} onChange={e => setEmail(e.target.value)} />
          <label className="checkbox"><input type="checkbox" checked={grantAccess} onChange={e => setGrantAccess(e.target.checked)} /> Enviar acesso ao app por SMS/WhatsApp (cliente sem e-mail)</label>
          {accessMessage && <p>{accessMessage}</p>}
          <button type="button" onClick={() => void addCustomer()}>Adicionar</button>
        </div></details>
      </>}
      <label>Serviço<select required value={form.service_id} onChange={e => setForm({ ...form, service_id: e.target.value })}><option value="">Selecione</option>{services.filter(x => x.active).map(x => <option value={x.id} key={x.id}>{x.name} · {money.format(x.price_cents / 100)}</option>)}</select></label>
      <label>Profissional<select required value={form.professional_id} onChange={e => setForm({ ...form, professional_id: e.target.value })}><option value="">Selecione</option>{professionals.filter(x => x.active).map(x => <option value={x.id} key={x.id}>{x.name}</option>)}</select></label>
      <label>Data e hora<input type="datetime-local" lang="pt-BR" required value={form.starts_at} onChange={e => setForm({ ...form, starts_at: e.target.value })} /></label>
      <label>Observações<textarea rows={3} value={form.notes} onChange={e => setForm({ ...form, notes: e.target.value })} /></label>
      <button className="primary" disabled={saving}>{saving ? 'Salvando…' : isClient ? 'Sugerir horário' : 'Confirmar agendamento'}</button>
    </form>
  </section>
}
