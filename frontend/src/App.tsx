import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { api, ApiError } from './api'
import { downloadICS, googleCalendarUrl } from './calendar'
import { fromE164BR, isValidCPF, isValidEmail, maskCPF, maskPhone, toE164BR } from './validation'
import type { AdminCustomerMatch, Appointment, Customer, Holiday, ImpersonationAuditEntry, MembershipOption, Professional, ProfessionalService, Report, Service, Tenant, TimeOff, User } from './types'

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
  const [initialChoice, setInitialChoice] = useState<{ preauthToken: string; memberships: MembershipOption[] } | null>(null)

  useEffect(() => {
    api.onSessionExpired(() => setUser(null))
    const { error, choice } = api.consumeOAuthRedirect()
    if (error) setOauthError(error)
    if (choice) setInitialChoice(choice)
    if (!api.isAuthenticated()) { setCheckingSession(false); return }
    api.me().then(setUser).catch(() => api.logout()).finally(() => setCheckingSession(false))
  }, [])

  if (checkingSession) return <div className="empty">Carregando…</div>
  if (!user) return <Login onLogin={setUser} initialError={oauthError} initialChoice={initialChoice} />
  if (user.role === 'superadmin') return <AdminPanel user={user} onImpersonate={setUser} onLogout={() => { api.logout(); setUser(null) }} />
  return <AgendaApp user={user} onLogout={() => { api.logout(); setUser(null) }} />
}

// A superadmin doesn't operate inside any single tenant — it only lists
// barbershops and "becomes" one's manager to actually do anything (see
// docs/api.md). Logging back out returns to this same login; to switch
// back to the admin view, log in again with the superadmin account.
function AdminPanel({ user, onImpersonate, onLogout }: { user: User; onImpersonate: (user: User) => void; onLogout: () => void }) {
  const [view, setView] = useState<'tenants' | 'customers' | 'audit' | 'profile'>('tenants')
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [customerQuery, setCustomerQuery] = useState('')
  const [customerResults, setCustomerResults] = useState<AdminCustomerMatch[]>([])
  const [searching, setSearching] = useState(false)
  const [auditEntries, setAuditEntries] = useState<ImpersonationAuditEntry[]>([])
  const [auditLoading, setAuditLoading] = useState(false)

  const load = useCallback(async () => {
    setLoading(true); setError('')
    try { setTenants(await api.listTenantsAdmin()) }
    catch (err) { setError(errorMessage(err)) }
    finally { setLoading(false) }
  }, [])

  useEffect(() => { void load() }, [load])

  // Search-as-you-type, same 400ms debounce as the tenant-name check on
  // signup — an empty query just clears the results instead of round-tripping.
  useEffect(() => {
    const query = customerQuery.trim()
    if (!query) { setCustomerResults([]); setSearching(false); return }
    setSearching(true)
    const timeout = setTimeout(async () => {
      try { setCustomerResults(await api.searchCustomersAdmin(query)) }
      catch (err) { setError(errorMessage(err)) }
      finally { setSearching(false) }
    }, 400)
    return () => clearTimeout(timeout)
  }, [customerQuery])

  // Reloads every time the tab is opened rather than caching — it's a
  // trail meant to be checked occasionally, freshness matters more than
  // saving a request.
  useEffect(() => {
    if (view !== 'audit') return
    setAuditLoading(true); setError('')
    api.listImpersonationAudit().then(setAuditEntries).catch(err => setError(errorMessage(err))).finally(() => setAuditLoading(false))
  }, [view])

  async function access(tenantId: string) {
    setError('')
    try { onImpersonate(await api.impersonateTenant(tenantId)) }
    catch (err) { setError(errorMessage(err)) }
  }

  return <div className="app-shell">
    <header>
      <div><span className="eyebrow">SUPERADMIN</span><h1>{view === 'customers' ? 'Clientes' : view === 'audit' ? 'Auditoria' : 'Barbearias'}</h1></div>
      <button className="avatar" title={`${user.name} · Perfil`} onClick={() => setView('profile')}>{user.name.slice(0, 2).toUpperCase()}</button>
    </header>
    <main>
      {view === 'profile' && <Profile user={user} onLogout={onLogout} onBack={() => setView('tenants')} />}

      {view === 'tenants' && <>
        {error && <div className="alert" role="alert">{error}<button onClick={() => setError('')}>×</button></div>}
        {loading ? <div className="empty">Carregando…</div> :
          tenants.length === 0 ? <div className="empty"><span>🏠</span><h2>Nenhuma barbearia</h2></div> :
          <ul className="tenant-list">{tenants.map(t => <li key={t.id}>
            <div><b>{t.name}</b><small>{t.slug}{!t.active ? ' · inativa' : ''}</small></div>
            <button onClick={() => void access(t.id)}>Acessar como gestor</button>
          </li>)}</ul>}
      </>}

      {view === 'customers' && <>
        {error && <div className="alert" role="alert">{error}<button onClick={() => setError('')}>×</button></div>}
        <label>Buscar por nome, celular ou e-mail
          <input value={customerQuery} onChange={e => setCustomerQuery(e.target.value)} placeholder="Ex: Maria, (11) 99999-0000…" autoFocus />
        </label>
        {searching ? <div className="empty">Buscando…</div> :
          !customerQuery.trim() ? <div className="empty"><span>🔎</span><p>Digite pra buscar clientes em todas as barbearias.</p></div> :
          customerResults.length === 0 ? <div className="empty"><span>🔎</span><h2>Nenhum cliente encontrado</h2></div> :
          <ul className="tenant-list">{customerResults.map(c => <li key={c.id}>
            <div><b>{c.name}</b><small>{c.tenant_name}{c.phone ? ` · ${c.phone}` : ''}{c.email ? ` · ${c.email}` : ''}{!c.active ? ' · inativo' : ''}</small></div>
            <button onClick={() => void access(c.tenant_id)}>Acessar como gestor</button>
          </li>)}</ul>}
      </>}

      {view === 'audit' && <>
        {error && <div className="alert" role="alert">{error}<button onClick={() => setError('')}>×</button></div>}
        {auditLoading ? <div className="empty">Carregando…</div> :
          auditEntries.length === 0 ? <div className="empty"><span>🕓</span><h2>Nenhum acesso registrado</h2></div> :
          <ul className="tenant-list">{auditEntries.map(a => <li key={a.id}>
            <div><b>{a.tenant_name}</b><small>{a.actor_name}{a.actor_email ? ` (${a.actor_email})` : ''} · {dateTime.format(new Date(a.created_at))}</small></div>
          </li>)}</ul>}
      </>}
    </main>
    {view !== 'profile' && <nav>
      <button className={view === 'tenants' ? 'active' : ''} onClick={() => setView('tenants')}><span>🏠</span>Barbearias</button>
      <button className={view === 'customers' ? 'active' : ''} onClick={() => setView('customers')}><span>🔎</span>Clientes</button>
      <button className={view === 'audit' ? 'active' : ''} onClick={() => setView('audit')}><span>🕓</span>Auditoria</button>
    </nav>}
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

type LoginMode = 'email' | 'phone' | 'recover' | 'signup' | 'choose-tenant'

function slugify(value: string) {
  return value.toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
}

function Login({ onLogin, initialError, initialChoice }: {
  onLogin: (user: User) => void; initialError?: string
  initialChoice?: { preauthToken: string; memberships: MembershipOption[] } | null
}) {
  const [mode, setMode] = useState<LoginMode>(initialChoice ? 'choose-tenant' : 'email')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState(initialError ?? '')
  const [info, setInfo] = useState('')
  const [loading, setLoading] = useState(false)
  const [signup, setSignup] = useState({ tenantName: '', managerName: '', email: '', password: '' })
  const [nameStatus, setNameStatus] = useState<'idle' | 'checking' | 'available' | 'taken'>('idle')
  const [choice, setChoice] = useState(initialChoice ?? null)

  useEffect(() => {
    const name = signup.tenantName.trim()
    if (!name) { setNameStatus('idle'); return }
    setNameStatus('checking')
    const timeout = setTimeout(async () => {
      try { setNameStatus((await api.checkTenantName(name)).available ? 'available' : 'taken') }
      catch { setNameStatus('idle') }
    }, 400)
    return () => clearTimeout(timeout)
  }, [signup.tenantName])

  function handleLoginResult(result: Awaited<ReturnType<typeof api.login>>) {
    if (result.kind === 'choice') { setChoice(result); setMode('choose-tenant'); return }
    onLogin(result.user)
  }

  async function submitEmail(event: FormEvent) {
    event.preventDefault(); setLoading(true); setError('')
    try { handleLoginResult(await api.login(email, password)) }
    catch (err) { setError(errorMessage(err)) }
    finally { setLoading(false) }
  }

  async function submitSignup(event: FormEvent) {
    event.preventDefault(); setError('')
    if (nameStatus === 'taken') { setError('Nome já em uso.'); return }
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
    try { handleLoginResult(await api.loginByPhone(toE164BR(phone), password)) }
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

  async function chooseMembership(membershipId: string) {
    if (!choice) return
    setLoading(true); setError('')
    try { onLogin(await api.selectMembership(choice.preauthToken, membershipId)) }
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

        {mode === 'choose-tenant' && choice && <>
          <p>Esse celular/e-mail tem acesso em mais de uma barbearia. Qual delas?</p>
          <ul className="tenant-list">
            {choice.memberships.map(m => <li key={m.membership_id} role="button" tabIndex={0} onClick={() => chooseMembership(m.membership_id)}>
              <div><b>{m.tenant_name}</b><small>{roleLabel[m.role] ?? m.role}</small></div>
            </li>)}
          </ul>
          <button type="button" className="link-button" onClick={() => { setChoice(null); setMode('email'); setError('') }}>Voltar para o login</button>
        </>}

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
            <label>Nome da barbearia
              <span className="field-status">
                <input required value={signup.tenantName} onChange={e => setSignup({ ...signup, tenantName: e.target.value })} />
                {nameStatus === 'available' && <span className="field-icon ok" aria-label="Nome disponível">✓</span>}
                {nameStatus === 'taken' && <span className="field-icon bad" aria-label="Nome já em uso">✗</span>}
              </span>
              {nameStatus === 'taken' && <small className="field-error">Nome já em uso</small>}
            </label>
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
  const [configView, setConfigView] = useState<'menu' | 'agenda' | 'tenant' | 'services' | 'professionals' | 'clients' | 'reports'>('menu')
  const [offset, setOffset] = useState(0)
  const [appointments, setAppointments] = useState<Appointment[]>([])
  const [services, setServices] = useState<Service[]>([])
  const [professionals, setProfessionals] = useState<Professional[]>([])
  const [customers, setCustomers] = useState<Customer[]>([])
  const [tenant, setTenant] = useState<Tenant | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [filterProfessional, setFilterProfessional] = useState('')
  const [filterStatus, setFilterStatus] = useState<Appointment['status'] | ''>('')
  const [filterService, setFilterService] = useState('')
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

  const filteredAppointments = useMemo(() => appointments.filter(item =>
    (!filterProfessional || item.professional_id === filterProfessional) &&
    (!filterStatus || item.status === filterStatus) &&
    (!filterService || item.service_id === filterService)
  ), [appointments, filterProfessional, filterStatus, filterService])

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
          <div><b>{offset === 0 ? 'Hoje' : dateTime.format(new Date(bounds.from)).split(' às')[0]}</b><small>{filteredAppointments.length} atendimento(s)</small></div>
          <button aria-label="Próximo dia" onClick={() => setOffset(v => v + 1)}>›</button>
        </section>
        {!isClient && <section className="filters">
          <select aria-label="Filtrar por profissional" value={filterProfessional} onChange={e => setFilterProfessional(e.target.value)}>
            <option value="">Profissional</option>
            {professionals.map(p => <option value={p.id} key={p.id}>{p.name}</option>)}
          </select>
          <select aria-label="Filtrar por status" value={filterStatus} onChange={e => setFilterStatus(e.target.value as Appointment['status'] | '')}>
            <option value="">Status</option>
            {(Object.keys(statusLabel) as Appointment['status'][]).map(s => <option value={s} key={s}>{statusLabel[s]}</option>)}
          </select>
          <select aria-label="Filtrar por serviço" value={filterService} onChange={e => setFilterService(e.target.value)}>
            <option value="">Serviço</option>
            {services.map(s => <option value={s.id} key={s.id}>{s.name}</option>)}
          </select>
        </section>}
        {loading ? <div className="empty">Carregando agenda…</div> :
          filteredAppointments.length === 0 ? <div className="empty"><span>✂</span><h2>Agenda livre</h2>
            <p>{appointments.length > 0 ? 'Nenhum compromisso encontrado com esses filtros.' : isClient ? 'Você ainda não tem horário marcado.' : 'Que tal criar o primeiro horário do dia?'}</p>
          </div> :
          <section className="appointments">{filteredAppointments.map(item => <article className={`card ${item.status}`} key={item.id}>
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
        {configView === 'menu' && <ConfigMenu onSelect={setConfigView} />}
        {configView === 'agenda' && <TenantConfig tenant={tenant} onSaved={t => setTenant(t)} onBack={() => setConfigView('menu')} />}
        {configView === 'tenant' && <TenantAccount tenant={tenant} onSaved={t => setTenant(t)} onBack={() => setConfigView('menu')} />}
        {configView === 'services' && <ServicesManager services={services} onCreated={s => setServices(v => [...v, s])}
          onUpdated={s => setServices(v => v.map(x => x.id === s.id ? s : x))} onBack={() => setConfigView('menu')} />}
        {configView === 'professionals' && <ProfessionalsManager professionals={professionals} services={services} onCreated={p => setProfessionals(v => [...v, p])}
          onUpdated={p => setProfessionals(v => v.map(x => x.id === p.id ? p : x))} onBack={() => setConfigView('menu')} />}
        {configView === 'clients' && <ClientsManager customers={customers} onCreated={c => setCustomers(v => [...v, c])}
          onUpdated={c => setCustomers(v => v.map(x => x.id === c.id ? c : x))} onBack={() => setConfigView('menu')} />}
        {configView === 'reports' && <ReportsView onBack={() => setConfigView('menu')} />}
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
        <button className={tab === 'config' ? 'active' : ''} onClick={() => { setTab('config'); setConfigView('menu') }}><span>⚙</span>Config</button>}
    </nav>
  </div>
}

function ConfigMenu({ onSelect }: { onSelect: (view: 'agenda' | 'tenant' | 'services' | 'professionals' | 'clients' | 'reports') => void }) {
  return <section className="form-page"><span className="eyebrow">CONFIGURAÇÕES</span><h2>Config</h2>
    <ul className="tenant-list">
      <li role="button" tabIndex={0} onClick={() => onSelect('tenant')}><div><b>Barbearia</b><small>Nome, slug e dados da conta</small></div></li>
      <li role="button" tabIndex={0} onClick={() => onSelect('agenda')}><div><b>Agenda</b><small>Autoagendamento e confirmação</small></div></li>
      <li role="button" tabIndex={0} onClick={() => onSelect('services')}><div><b>Serviços</b><small>Cadastro de serviços</small></div></li>
      <li role="button" tabIndex={0} onClick={() => onSelect('professionals')}><div><b>Profissionais</b><small>Cadastro de profissionais</small></div></li>
      <li role="button" tabIndex={0} onClick={() => onSelect('clients')}><div><b>Clientes</b><small>Cadastro de clientes</small></div></li>
      <li role="button" tabIndex={0} onClick={() => onSelect('reports')}><div><b>Relatórios</b><small>Ocupação, faturamento e retenção</small></div></li>
    </ul>
  </section>
}

function TenantConfig({ tenant, onSaved, onBack }: { tenant: Tenant; onSaved: (tenant: Tenant) => void; onBack: () => void }) {
  const [selfScheduling, setSelfScheduling] = useState(tenant.self_scheduling_enabled)
  const [autoConfirm, setAutoConfirm] = useState(tenant.auto_confirm_appointments)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [holidays, setHolidays] = useState<Holiday[]>([])
  const [holidaysLoading, setHolidaysLoading] = useState(true)
  const [holidaysError, setHolidaysError] = useState('')
  const [newHoliday, setNewHoliday] = useState({ date: '', name: '' })
  const [holidaySaving, setHolidaySaving] = useState(false)

  useEffect(() => {
    api.holidays().then(setHolidays).catch(err => setHolidaysError(errorMessage(err))).finally(() => setHolidaysLoading(false))
  }, [])

  async function save() {
    setSaving(true); setError('')
    try { onSaved(await api.updateTenant({ name: tenant.name, slug: tenant.slug, self_scheduling_enabled: selfScheduling, auto_confirm_appointments: autoConfirm })) }
    catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }

  async function addHoliday(event: FormEvent) {
    event.preventDefault(); setHolidaysError(''); setHolidaySaving(true)
    try {
      const created = await api.createHoliday({ date: newHoliday.date, name: newHoliday.name })
      setHolidays(v => [...v, created].sort((a, b) => a.date.localeCompare(b.date)))
      setNewHoliday({ date: '', name: '' })
    }
    catch (err) { setHolidaysError(errorMessage(err)) }
    finally { setHolidaySaving(false) }
  }

  async function removeHoliday(id: string) {
    setHolidaysError('')
    try { await api.deleteHoliday(id); setHolidays(v => v.filter(h => h.id !== id)) }
    catch (err) { setHolidaysError(errorMessage(err)) }
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

    <h2>Feriados</h2>
    {holidaysError && <div className="alert">{holidaysError}</div>}
    {holidaysLoading ? <p>Carregando…</p> : <>
      <p>Nenhum agendamento é aceito nessas datas, para nenhum profissional.</p>
      {holidays.length === 0 ? <p>Nenhum feriado cadastrado.</p> : <ul className="tenant-list">
        {holidays.map(h => <li key={h.id}>
          <div><b>{new Date(h.date + 'T00:00:00').toLocaleDateString('pt-BR', { day: '2-digit', month: 'long', year: 'numeric' })}</b>{h.name && <small> · {h.name}</small>}</div>
          <button type="button" onClick={() => void removeHoliday(h.id)}>Remover</button>
        </li>)}
      </ul>}
      <form onSubmit={addHoliday} className="quick">
        <label>Data<input type="date" lang="pt-BR" required value={newHoliday.date} onChange={e => setNewHoliday({ ...newHoliday, date: e.target.value })} /></label>
        <input placeholder="Nome (opcional, ex: Natal)" value={newHoliday.name} onChange={e => setNewHoliday({ ...newHoliday, name: e.target.value })} />
        <button disabled={holidaySaving}>{holidaySaving ? 'Adicionando…' : 'Adicionar feriado'}</button>
      </form>
    </>}

    <button type="button" className="link-button" onClick={onBack}>Voltar</button>
  </section>
}

// Editing name reuses the same real-time availability check as the
// barbershop signup form, but skips it while the typed name still matches
// the tenant's own current name — otherwise submitting unchanged would
// falsely show "já em uso" against itself.
function TenantAccount({ tenant, onSaved, onBack }: { tenant: Tenant; onSaved: (tenant: Tenant) => void; onBack: () => void }) {
  const [name, setName] = useState(tenant.name)
  const [slug, setSlug] = useState(tenant.slug)
  const [nameStatus, setNameStatus] = useState<'idle' | 'checking' | 'available' | 'taken'>('idle')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [info, setInfo] = useState('')

  useEffect(() => {
    const trimmed = name.trim()
    if (!trimmed || trimmed.toLowerCase() === tenant.name.toLowerCase()) { setNameStatus('idle'); return }
    setNameStatus('checking')
    const timeout = setTimeout(async () => {
      try { setNameStatus((await api.checkTenantName(trimmed)).available ? 'available' : 'taken') }
      catch { setNameStatus('idle') }
    }, 400)
    return () => clearTimeout(timeout)
  }, [name, tenant.name])

  async function save(event: FormEvent) {
    event.preventDefault(); setError(''); setInfo('')
    if (nameStatus === 'taken') { setError('Nome já em uso.'); return }
    setSaving(true)
    try {
      const updated = await api.updateTenant({
        name: name.trim(), slug: slug.trim().toLowerCase(),
        self_scheduling_enabled: tenant.self_scheduling_enabled, auto_confirm_appointments: tenant.auto_confirm_appointments
      })
      onSaved(updated)
      setInfo('Dados salvos.')
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }

  return <section className="form-page"><span className="eyebrow">CONFIGURAÇÕES</span><h2>Barbearia</h2>
    {error && <div className="alert" role="alert">{error}</div>}
    {info && <p>{info}</p>}
    <form onSubmit={save}>
      <label>Nome da barbearia
        <span className="field-status">
          <input required value={name} onChange={e => setName(e.target.value)} />
          {nameStatus === 'available' && <span className="field-icon ok" aria-label="Nome disponível">✓</span>}
          {nameStatus === 'taken' && <span className="field-icon bad" aria-label="Nome já em uso">✗</span>}
        </span>
        {nameStatus === 'taken' && <small className="field-error">Nome já em uso</small>}
      </label>
      <label>Slug (usado em links de cadastro)<input required value={slug} onChange={e => setSlug(e.target.value.toLowerCase())} /></label>
      <button className="primary" disabled={saving}>{saving ? 'Salvando…' : 'Salvar'}</button>
    </form>
    <div className="profile-info">
      <div><small>Status</small><b>{tenant.active ? 'Ativa' : 'Inativa'}</b></div>
      <div><small>Criada em</small><b>{new Date(tenant.created_at).toLocaleDateString('pt-BR')}</b></div>
    </div>
    <button type="button" className="link-button" onClick={onBack}>Voltar</button>
  </section>
}

type ServiceScreen = { name: 'list' } | { name: 'detail'; id: string } | { name: 'edit'; id: string } | { name: 'create' }

function ServicesManager({ services, onCreated, onUpdated, onBack }: { services: Service[]; onCreated: (service: Service) => void; onUpdated: (service: Service) => void; onBack: () => void }) {
  const [screen, setScreen] = useState<ServiceScreen>({ name: 'list' })
  const [name, setName] = useState(''); const [duration, setDuration] = useState('30'); const [price, setPrice] = useState('')
  const [saving, setSaving] = useState(false); const [error, setError] = useState('')
  const [editName, setEditName] = useState(''); const [editDuration, setEditDuration] = useState(''); const [editPrice, setEditPrice] = useState('')
  const [editSaving, setEditSaving] = useState(false)
  const [togglingId, setTogglingId] = useState<string | null>(null)

  function startCreate() {
    setName(''); setDuration('30'); setPrice(''); setError('')
    setScreen({ name: 'create' })
  }

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
      setScreen({ name: 'list' })
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }

  function startEdit(service: Service) {
    setEditName(service.name)
    setEditDuration(String(service.duration_minutes))
    setEditPrice((service.price_cents / 100).toFixed(2).replace('.', ','))
    setError('')
    setScreen({ name: 'edit', id: service.id })
  }

  async function submitEdit(event: FormEvent) {
    event.preventDefault(); setError('')
    if (screen.name !== 'edit') return
    const service = services.find(s => s.id === screen.id); if (!service) return
    const durationMinutes = Number(editDuration)
    const priceCents = Math.round(Number(editPrice.replace(',', '.')) * 100)
    if (!Number.isFinite(durationMinutes) || durationMinutes < 5 || durationMinutes > 480) {
      setError('Duração deve ser entre 5 e 480 minutos.'); return
    }
    if (!editPrice.trim() || !Number.isFinite(priceCents) || priceCents < 0) { setError('Informe um preço válido.'); return }
    setEditSaving(true)
    try {
      onUpdated(await api.updateService(service.id, { name: editName.trim(), duration_minutes: durationMinutes, price_cents: priceCents, active: service.active }))
      setScreen({ name: 'detail', id: service.id })
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setEditSaving(false) }
  }

  async function toggleActive(service: Service) {
    setError(''); setTogglingId(service.id)
    try {
      onUpdated(await api.updateService(service.id, { name: service.name, duration_minutes: service.duration_minutes, price_cents: service.price_cents, active: !service.active }))
      setScreen({ name: 'list' })
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setTogglingId(null) }
  }

  const detailService = screen.name === 'detail' ? services.find(s => s.id === screen.id) : undefined

  return <section className="form-page">
    <span className="eyebrow">CONFIGURAÇÕES</span><h2>Serviços</h2>
    {error && <div className="alert">{error}</div>}

    {screen.name === 'list' && <>
      {services.length === 0 ? <p>Nenhum serviço cadastrado.</p> : <ul className="tenant-list">
        {services.map(s => <li key={s.id} role="button" tabIndex={0} onClick={() => setScreen({ name: 'detail', id: s.id })}>
          <div><b>{s.name}</b><small>{s.duration_minutes} min · {money.format(s.price_cents / 100)}{!s.active ? ' · inativo' : ''}</small></div>
        </li>)}
      </ul>}
      <button type="button" className="link-button" onClick={onBack}>Voltar</button>
      <button className="fab" aria-label="Novo serviço" title="Novo serviço" onClick={startCreate}>+</button>
    </>}

    {detailService && <>
      <h3>{detailService.name}</h3>
      <p>{detailService.duration_minutes} min · {money.format(detailService.price_cents / 100)}{!detailService.active ? ' · inativo' : ''}</p>
      <div className="quick">
        <button type="button" onClick={() => startEdit(detailService)}>Editar</button>
        <button type="button" disabled={togglingId === detailService.id} onClick={() => toggleActive(detailService)}>
          {togglingId === detailService.id ? 'Salvando…' : detailService.active ? 'Excluir' : 'Reativar'}
        </button>
      </div>
      <button type="button" className="link-button" onClick={() => setScreen({ name: 'list' })}>Voltar</button>
    </>}

    {screen.name === 'edit' && <form onSubmit={submitEdit} className="quick">
      <input placeholder="Nome" required value={editName} onChange={e => setEditName(e.target.value)} />
      <input type="number" min={5} max={480} placeholder="Duração (minutos)" required value={editDuration} onChange={e => setEditDuration(e.target.value)} />
      <input type="text" inputMode="decimal" placeholder="Preço (R$)" required value={editPrice} onChange={e => setEditPrice(e.target.value)} />
      <button disabled={editSaving}>{editSaving ? 'Salvando…' : 'Salvar'}</button>
      <button type="button" onClick={() => setScreen({ name: 'detail', id: screen.id })}>Cancelar</button>
    </form>}

    {screen.name === 'create' && <form onSubmit={submit} className="quick">
      <input placeholder="Nome" required value={name} onChange={e => setName(e.target.value)} />
      <input type="number" min={5} max={480} placeholder="Duração (minutos)" required value={duration} onChange={e => setDuration(e.target.value)} />
      <input type="text" inputMode="decimal" placeholder="Preço (R$)" required value={price} onChange={e => setPrice(e.target.value)} />
      <button disabled={saving}>{saving ? 'Adicionando…' : 'Adicionar serviço'}</button>
      <button type="button" onClick={() => setScreen({ name: 'list' })}>Cancelar</button>
    </form>}
  </section>
}

type ProfessionalScreen = { name: 'list' } | { name: 'detail'; id: string } | { name: 'edit'; id: string } | { name: 'create' }

type SpecialtyEntry = { enabled: boolean; price: string; duration: string }

function ProfessionalsManager({ professionals, services, onCreated, onUpdated, onBack }: { professionals: Professional[]; services: Service[]; onCreated: (professional: Professional) => void; onUpdated: (professional: Professional) => void; onBack: () => void }) {
  const [screen, setScreen] = useState<ProfessionalScreen>({ name: 'list' })
  const [name, setName] = useState(''); const [phone, setPhone] = useState('')
  const [email, setEmail] = useState(''); const [cpf, setCpf] = useState('')
  const [saving, setSaving] = useState(false); const [error, setError] = useState('')
  const [editName, setEditName] = useState(''); const [editPhone, setEditPhone] = useState('')
  const [editEmail, setEditEmail] = useState(''); const [editCpf, setEditCpf] = useState('')
  const [editSaving, setEditSaving] = useState(false)
  const [togglingId, setTogglingId] = useState<string | null>(null)
  const [granting, setGranting] = useState(false)
  const [grantPhone, setGrantPhone] = useState('')
  const [grantMessage, setGrantMessage] = useState('')
  const [grantSaving, setGrantSaving] = useState(false)
  const [specialtiesOpen, setSpecialtiesOpen] = useState(false)
  const [specialtiesLoading, setSpecialtiesLoading] = useState(false)
  const [specialtiesSaving, setSpecialtiesSaving] = useState(false)
  const [specialtiesError, setSpecialtiesError] = useState('')
  const [specialtiesEntries, setSpecialtiesEntries] = useState<Record<string, SpecialtyEntry>>({})

  async function startSpecialties(professional: Professional) {
    setSpecialtiesOpen(true); setSpecialtiesError(''); setSpecialtiesLoading(true)
    try {
      const current = await api.getProfessionalServices(professional.id)
      const byServiceId = new Map(current.map(item => [item.service_id, item]))
      const entries: Record<string, SpecialtyEntry> = {}
      for (const service of services) {
        const existing = byServiceId.get(service.id)
        entries[service.id] = {
          enabled: !!existing,
          price: existing?.price_cents_override != null ? (existing.price_cents_override / 100).toFixed(2).replace('.', ',') : '',
          duration: existing?.duration_minutes_override != null ? String(existing.duration_minutes_override) : '',
        }
      }
      setSpecialtiesEntries(entries)
    }
    catch (err) { setSpecialtiesError(errorMessage(err)) }
    finally { setSpecialtiesLoading(false) }
  }

  async function submitSpecialties(event: FormEvent) {
    event.preventDefault()
    if (screen.name !== 'detail') return
    setSpecialtiesError('')
    const entries: ProfessionalService[] = []
    for (const service of services) {
      const entry = specialtiesEntries[service.id]
      if (!entry?.enabled) continue
      let priceCentsOverride: number | null = null
      if (entry.price.trim()) {
        priceCentsOverride = Math.round(Number(entry.price.replace(',', '.')) * 100)
        if (!Number.isFinite(priceCentsOverride) || priceCentsOverride < 0) {
          setSpecialtiesError(`Preço inválido para "${service.name}".`); return
        }
      }
      let durationMinutesOverride: number | null = null
      if (entry.duration.trim()) {
        durationMinutesOverride = Number(entry.duration)
        if (!Number.isFinite(durationMinutesOverride) || durationMinutesOverride <= 0) {
          setSpecialtiesError(`Duração inválida para "${service.name}".`); return
        }
      }
      entries.push({ service_id: service.id, price_cents_override: priceCentsOverride, duration_minutes_override: durationMinutesOverride })
    }
    setSpecialtiesSaving(true)
    try {
      await api.setProfessionalServices(screen.id, entries)
      setSpecialtiesOpen(false)
    }
    catch (err) { setSpecialtiesError(errorMessage(err)) }
    finally { setSpecialtiesSaving(false) }
  }

  function updateSpecialty(serviceId: string, patch: Partial<SpecialtyEntry>) {
    setSpecialtiesEntries(v => ({ ...v, [serviceId]: { ...v[serviceId], ...patch } }))
  }

  function startCreate() {
    setName(''); setPhone(''); setEmail(''); setCpf(''); setError('')
    setScreen({ name: 'create' })
  }

  async function submit(event: FormEvent) {
    event.preventDefault(); setError('')
    if (email.trim() && !isValidEmail(email.trim())) { setError('Informe um e-mail válido ou deixe em branco.'); return }
    if (cpf.trim() && !isValidCPF(cpf)) { setError('CPF inválido.'); return }
    setSaving(true)
    try {
      onCreated(await api.createProfessional({ name, phone: toE164BR(phone), email, cpf: cpf.replace(/\D/g, '') }))
      setScreen({ name: 'list' })
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }

  function startEdit(professional: Professional) {
    setEditName(professional.name)
    setEditPhone(professional.phone ? fromE164BR(professional.phone) : '')
    setEditEmail(professional.email ?? '')
    setEditCpf(professional.cpf ?? '')
    setError('')
    setScreen({ name: 'edit', id: professional.id })
  }

  async function submitEdit(event: FormEvent) {
    event.preventDefault(); setError('')
    if (screen.name !== 'edit') return
    const professional = professionals.find(p => p.id === screen.id); if (!professional) return
    if (editEmail.trim() && !isValidEmail(editEmail.trim())) { setError('Informe um e-mail válido ou deixe em branco.'); return }
    if (editCpf.trim() && !isValidCPF(editCpf)) { setError('CPF inválido.'); return }
    setEditSaving(true)
    try {
      onUpdated(await api.updateProfessional(professional.id, {
        name: editName.trim(), phone: toE164BR(editPhone), email: editEmail.trim(), cpf: editCpf.replace(/\D/g, ''), active: professional.active
      }))
      setScreen({ name: 'detail', id: professional.id })
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setEditSaving(false) }
  }

  async function toggleActive(professional: Professional) {
    setError(''); setTogglingId(professional.id)
    try {
      onUpdated(await api.updateProfessional(professional.id, {
        name: professional.name, phone: professional.phone, email: professional.email, cpf: professional.cpf, active: !professional.active
      }))
      setScreen({ name: 'list' })
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setTogglingId(null) }
  }

  function startGrant(professional: Professional) {
    setGranting(true)
    setGrantPhone(professional.phone ? fromE164BR(professional.phone) : '')
    setGrantMessage(''); setError('')
  }

  async function submitGrant(event: FormEvent) {
    event.preventDefault()
    if (screen.name !== 'detail') return
    setError(''); setGrantMessage(''); setGrantSaving(true)
    try { setGrantMessage((await api.grantProfessionalAccess(screen.id, toE164BR(grantPhone))).message) }
    catch (err) { setError(errorMessage(err)) }
    finally { setGrantSaving(false) }
  }

  const detailProfessional = screen.name === 'detail' ? professionals.find(p => p.id === screen.id) : undefined

  return <section className="form-page">
    <span className="eyebrow">CONFIGURAÇÕES</span><h2>Profissionais</h2>
    {error && <div className="alert">{error}</div>}

    {screen.name === 'list' && <>
      {professionals.length === 0 ? <p>Nenhum profissional cadastrado.</p> : <ul className="tenant-list">
        {professionals.map(p => <li key={p.id} role="button" tabIndex={0} onClick={() => setScreen({ name: 'detail', id: p.id })}>
          <div><b>{p.name}</b>{!p.active && <small> · inativo</small>}{(p.email || p.phone) && <small> · {[p.email, p.phone].filter(Boolean).join(' · ')}</small>}</div>
        </li>)}
      </ul>}
      <button type="button" className="link-button" onClick={onBack}>Voltar</button>
      <button className="fab" aria-label="Novo profissional" title="Novo profissional" onClick={startCreate}>+</button>
    </>}

    {detailProfessional && <>
      <h3>{detailProfessional.name}</h3>
      <p>{!detailProfessional.active && 'Inativo'}{(detailProfessional.email || detailProfessional.phone) && ((!detailProfessional.active ? ' · ' : '') + [detailProfessional.email, detailProfessional.phone].filter(Boolean).join(' · '))}</p>
      <div className="quick">
        <button type="button" onClick={() => startEdit(detailProfessional)}>Editar</button>
        <button type="button" disabled={togglingId === detailProfessional.id} onClick={() => toggleActive(detailProfessional)}>
          {togglingId === detailProfessional.id ? 'Salvando…' : detailProfessional.active ? 'Excluir' : 'Reativar'}
        </button>
        <button type="button" onClick={() => startGrant(detailProfessional)}>Conceder acesso</button>
        <button type="button" onClick={() => startSpecialties(detailProfessional)}>Especialidades e preços</button>
      </div>
      {granting && <form onSubmit={submitGrant} className="quick">
        <p>Enviar senha de acesso ao app para <b>{detailProfessional.name}</b> por SMS/WhatsApp:</p>
        {grantMessage && <p>{grantMessage}</p>}
        <input type="tel" placeholder="(11) 99999-0000" required value={grantPhone} onChange={e => setGrantPhone(maskPhone(e.target.value))} maxLength={16} />
        <button disabled={grantSaving}>{grantSaving ? 'Enviando…' : 'Enviar senha'}</button>
        <button type="button" onClick={() => setGranting(false)}>Fechar</button>
      </form>}
      {specialtiesOpen && <form onSubmit={submitSpecialties} className="quick">
        <p>Serviços que <b>{detailProfessional.name}</b> pode realizar. Sem nenhum marcado, executa qualquer serviço ativo com preço/duração padrão.</p>
        {specialtiesError && <div className="alert">{specialtiesError}</div>}
        {specialtiesLoading ? <p>Carregando…</p> : services.map(service => {
          const entry = specialtiesEntries[service.id] ?? { enabled: false, price: '', duration: '' }
          return <div key={service.id} className="specialty-row">
            <label>
              <input type="checkbox" checked={entry.enabled} onChange={e => updateSpecialty(service.id, { enabled: e.target.checked })} />
              {' '}{service.name} <small>({service.duration_minutes} min · {money.format(service.price_cents / 100)})</small>
            </label>
            {entry.enabled && <div className="specialty-overrides">
              <input type="text" inputMode="decimal" placeholder="Preço (R$, opcional)" value={entry.price} onChange={e => updateSpecialty(service.id, { price: e.target.value })} />
              <input type="number" min={5} max={480} placeholder="Duração (min, opcional)" value={entry.duration} onChange={e => updateSpecialty(service.id, { duration: e.target.value })} />
            </div>}
          </div>
        })}
        <button disabled={specialtiesSaving || specialtiesLoading}>{specialtiesSaving ? 'Salvando…' : 'Salvar especialidades'}</button>
        <button type="button" onClick={() => setSpecialtiesOpen(false)}>Fechar</button>
      </form>}
      <button type="button" className="link-button" onClick={() => { setGranting(false); setSpecialtiesOpen(false); setScreen({ name: 'list' }) }}>Voltar</button>
    </>}

    {screen.name === 'edit' && <form onSubmit={submitEdit} className="quick">
      <input placeholder="Nome" required value={editName} onChange={e => setEditName(e.target.value)} />
      <input type="tel" placeholder="(11) 99999-0000 (opcional)" value={editPhone} onChange={e => setEditPhone(maskPhone(e.target.value))} maxLength={16} />
      <input type="email" placeholder="E-mail (opcional)" value={editEmail} onChange={e => setEditEmail(e.target.value)} />
      <input placeholder="CPF (opcional)" value={editCpf} onChange={e => setEditCpf(maskCPF(e.target.value))} maxLength={14} />
      <button disabled={editSaving}>{editSaving ? 'Salvando…' : 'Salvar'}</button>
      <button type="button" onClick={() => setScreen({ name: 'detail', id: screen.id })}>Cancelar</button>
    </form>}

    {screen.name === 'create' && <form onSubmit={submit} className="quick">
      <input placeholder="Nome" required value={name} onChange={e => setName(e.target.value)} />
      <input type="tel" placeholder="(11) 99999-0000 (opcional)" value={phone} onChange={e => setPhone(maskPhone(e.target.value))} maxLength={16} />
      <input type="email" placeholder="E-mail (opcional)" value={email} onChange={e => setEmail(e.target.value)} />
      <input placeholder="CPF (opcional)" value={cpf} onChange={e => setCpf(maskCPF(e.target.value))} maxLength={14} />
      <button disabled={saving}>{saving ? 'Adicionando…' : 'Adicionar profissional'}</button>
      <button type="button" onClick={() => setScreen({ name: 'list' })}>Cancelar</button>
    </form>}
  </section>
}

type ClientScreen = { name: 'list' } | { name: 'detail'; id: string } | { name: 'edit'; id: string } | { name: 'create' }

function ClientsManager({ customers, onCreated, onUpdated, onBack }: { customers: Customer[]; onCreated: (customer: Customer) => void; onUpdated: (customer: Customer) => void; onBack: () => void }) {
  const [screen, setScreen] = useState<ClientScreen>({ name: 'list' })
  const [name, setName] = useState(''); const [phone, setPhone] = useState(''); const [email, setEmail] = useState('')
  const [saving, setSaving] = useState(false); const [error, setError] = useState('')
  const [editName, setEditName] = useState(''); const [editPhone, setEditPhone] = useState(''); const [editEmail, setEditEmail] = useState('')
  const [editSaving, setEditSaving] = useState(false)
  const [togglingId, setTogglingId] = useState<string | null>(null)

  function startCreate() {
    setName(''); setPhone(''); setEmail(''); setError('')
    setScreen({ name: 'create' })
  }

  async function submit(event: FormEvent) {
    event.preventDefault(); setError('')
    if (email.trim() && !isValidEmail(email.trim())) { setError('Informe um e-mail válido ou deixe em branco.'); return }
    setSaving(true)
    try {
      onCreated(await api.createCustomer({ name, phone: toE164BR(phone), email }))
      setScreen({ name: 'list' })
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setSaving(false) }
  }

  function startEdit(customer: Customer) {
    setEditName(customer.name)
    setEditPhone(customer.phone ? fromE164BR(customer.phone) : '')
    setEditEmail(customer.email ?? '')
    setError('')
    setScreen({ name: 'edit', id: customer.id })
  }

  async function submitEdit(event: FormEvent) {
    event.preventDefault(); setError('')
    if (screen.name !== 'edit') return
    const customer = customers.find(c => c.id === screen.id); if (!customer) return
    if (editEmail.trim() && !isValidEmail(editEmail.trim())) { setError('Informe um e-mail válido ou deixe em branco.'); return }
    setEditSaving(true)
    try {
      onUpdated(await api.updateCustomer(customer.id, { name: editName.trim(), phone: toE164BR(editPhone), email: editEmail.trim(), active: customer.active }))
      setScreen({ name: 'detail', id: customer.id })
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setEditSaving(false) }
  }

  async function toggleActive(customer: Customer) {
    setError(''); setTogglingId(customer.id)
    try {
      onUpdated(await api.updateCustomer(customer.id, { name: customer.name, phone: customer.phone, email: customer.email, active: !customer.active }))
      setScreen({ name: 'list' })
    }
    catch (err) { setError(errorMessage(err)) }
    finally { setTogglingId(null) }
  }

  const detailCustomer = screen.name === 'detail' ? customers.find(c => c.id === screen.id) : undefined

  return <section className="form-page">
    <span className="eyebrow">CONFIGURAÇÕES</span><h2>Clientes</h2>
    {error && <div className="alert">{error}</div>}

    {screen.name === 'list' && <>
      {customers.length === 0 ? <p>Nenhum cliente cadastrado.</p> : <ul className="tenant-list">
        {customers.map(c => <li key={c.id} role="button" tabIndex={0} onClick={() => setScreen({ name: 'detail', id: c.id })}>
          <div><b>{c.name}</b>{!c.active && <small> · inativo</small>}{(c.email || c.phone) && <small> · {[c.email, c.phone].filter(Boolean).join(' · ')}</small>}</div>
        </li>)}
      </ul>}
      <button type="button" className="link-button" onClick={onBack}>Voltar</button>
      <button className="fab" aria-label="Novo cliente" title="Novo cliente" onClick={startCreate}>+</button>
    </>}

    {detailCustomer && <>
      <h3>{detailCustomer.name}</h3>
      <p>{!detailCustomer.active && 'Inativo'}{(detailCustomer.email || detailCustomer.phone) && ((!detailCustomer.active ? ' · ' : '') + [detailCustomer.email, detailCustomer.phone].filter(Boolean).join(' · '))}</p>
      <div className="quick">
        <button type="button" onClick={() => startEdit(detailCustomer)}>Editar</button>
        <button type="button" disabled={togglingId === detailCustomer.id} onClick={() => toggleActive(detailCustomer)}>
          {togglingId === detailCustomer.id ? 'Salvando…' : detailCustomer.active ? 'Excluir' : 'Reativar'}
        </button>
      </div>
      <button type="button" className="link-button" onClick={() => setScreen({ name: 'list' })}>Voltar</button>
    </>}

    {screen.name === 'edit' && <form onSubmit={submitEdit} className="quick">
      <input placeholder="Nome" required value={editName} onChange={e => setEditName(e.target.value)} />
      <input type="tel" placeholder="(11) 99999-0000 (opcional)" value={editPhone} onChange={e => setEditPhone(maskPhone(e.target.value))} maxLength={16} />
      <input type="email" placeholder="E-mail (opcional)" value={editEmail} onChange={e => setEditEmail(e.target.value)} />
      <button disabled={editSaving}>{editSaving ? 'Salvando…' : 'Salvar'}</button>
      <button type="button" onClick={() => setScreen({ name: 'detail', id: screen.id })}>Cancelar</button>
    </form>}

    {screen.name === 'create' && <form onSubmit={submit} className="quick">
      <input placeholder="Nome" required value={name} onChange={e => setName(e.target.value)} />
      <input type="tel" placeholder="(11) 99999-0000 (opcional)" value={phone} onChange={e => setPhone(maskPhone(e.target.value))} maxLength={16} />
      <input type="email" placeholder="E-mail (opcional)" value={email} onChange={e => setEmail(e.target.value)} />
      <button disabled={saving}>{saving ? 'Adicionando…' : 'Adicionar cliente'}</button>
      <button type="button" onClick={() => setScreen({ name: 'list' })}>Cancelar</button>
    </form>}
  </section>
}

function ReportsView({ onBack }: { onBack: () => void }) {
  const now = new Date()
  const [from, setFrom] = useState(new Date(now.getFullYear(), now.getMonth(), 1).toISOString().slice(0, 10))
  const [to, setTo] = useState(new Date(now.getFullYear(), now.getMonth() + 1, 1).toISOString().slice(0, 10))
  const [report, setReport] = useState<Report | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true); setError('')
    try { setReport(await api.report(new Date(from).toISOString(), new Date(to).toISOString())) }
    catch (err) { setError(errorMessage(err)) }
    finally { setLoading(false) }
  }, [from, to])

  useEffect(() => { void load() }, [load])

  return <section className="form-page"><span className="eyebrow">CONFIGURAÇÕES</span><h2>Relatórios</h2>
    {error && <div className="alert">{error}</div>}
    <div className="quick">
      <label>De<input type="date" lang="pt-BR" value={from} onChange={e => setFrom(e.target.value)} /></label>
      <label>Até<input type="date" lang="pt-BR" value={to} onChange={e => setTo(e.target.value)} /></label>
    </div>
    {loading ? <p>Carregando…</p> : report && <>
      <h3>Ocupação</h3>
      <p>Taxa geral: <b>{(report.occupancy.overall_rate * 100).toFixed(0)}%</b></p>
      {report.occupancy.by_professional.length === 0 ? <p>Nenhum profissional com jornada configurada no período.</p> : <ul className="tenant-list">
        {report.occupancy.by_professional.map(p => <li key={p.professional_id}>
          <div><b>{p.professional_name}</b><small>{Math.round(p.booked_minutes / 60)}h ocupadas de {Math.round(p.available_minutes / 60)}h disponíveis · {(p.rate * 100).toFixed(0)}%</small></div>
        </li>)}
      </ul>}

      <h3>Faturamento</h3>
      <p>Total: <b>{money.format(report.revenue.total_cents / 100)}</b></p>
      {report.revenue.by_professional.length === 0 ? <p>Nenhum atendimento concluído no período.</p> : <ul className="tenant-list">
        {report.revenue.by_professional.map(p => <li key={p.professional_id}>
          <div><b>{p.professional_name}</b><small>{money.format(p.total_cents / 100)}</small></div>
        </li>)}
      </ul>}

      <h3>Retenção</h3>
      <p>{report.retention.returning_customers} de {report.retention.total_customers} clientes atendidos já eram clientes antes do período (<b>{(report.retention.rate * 100).toFixed(0)}%</b>).</p>
    </>}
    <button type="button" className="link-button" onClick={onBack}>Voltar</button>
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
