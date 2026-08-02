import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { api } from './api'
import type { Appointment, Customer, Professional, Service } from './types'

const money = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })
const dateTime = new Intl.DateTimeFormat('pt-BR', { weekday: 'short', day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })
const statusLabel = { scheduled: 'Agendado', confirmed: 'Confirmado', completed: 'Concluído', cancelled: 'Cancelado' }

function dayBounds(offset = 0) {
  const from = new Date(); from.setDate(from.getDate() + offset); from.setHours(0, 0, 0, 0)
  const to = new Date(from); to.setDate(to.getDate() + 1)
  return { from: from.toISOString(), to: to.toISOString() }
}

export default function App() {
  const [tab, setTab] = useState<'agenda' | 'new'>('agenda')
  const [offset, setOffset] = useState(0)
  const [appointments, setAppointments] = useState<Appointment[]>([])
  const [services, setServices] = useState<Service[]>([])
  const [professionals, setProfessionals] = useState<Professional[]>([])
  const [customers, setCustomers] = useState<Customer[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const bounds = useMemo(() => dayBounds(offset), [offset])

  const load = useCallback(async () => {
    setLoading(true); setError('')
    try {
      const [agenda, serviceList, professionalList, customerList] = await Promise.all([
        api.appointments(bounds.from, bounds.to), api.services(), api.professionals(), api.customers()
      ])
      setAppointments(agenda); setServices(serviceList); setProfessionals(professionalList); setCustomers(customerList)
    } catch (err) { setError(err instanceof Error ? err.message : 'Erro inesperado') }
    finally { setLoading(false) }
  }, [bounds])

  useEffect(() => { void load() }, [load])

  async function changeStatus(item: Appointment, status: Appointment['status']) {
    try { await api.updateStatus(item.id, status); await load() }
    catch (err) { setError(err instanceof Error ? err.message : 'Erro inesperado') }
  }

  return <div className="app-shell">
    <header>
      <div><span className="eyebrow">BARBERFLOW</span><h1>Sua agenda</h1></div>
      <div className="avatar">BF</div>
    </header>

    <main>
      {error && <div className="alert" role="alert">{error}<button onClick={() => setError('')}>×</button></div>}
      {tab === 'agenda' ? <>
        <section className="date-nav">
          <button aria-label="Dia anterior" onClick={() => setOffset(v => v - 1)}>‹</button>
          <div><b>{offset === 0 ? 'Hoje' : dateTime.format(new Date(bounds.from)).split(' às')[0]}</b><small>{appointments.length} atendimento(s)</small></div>
          <button aria-label="Próximo dia" onClick={() => setOffset(v => v + 1)}>›</button>
        </section>
        {loading ? <div className="empty">Carregando agenda…</div> :
          appointments.length === 0 ? <div className="empty"><span>✂</span><h2>Agenda livre</h2><p>Que tal criar o primeiro horário do dia?</p><button className="primary" onClick={() => setTab('new')}>Novo agendamento</button></div> :
          <section className="appointments">{appointments.map(item => <article className={`card ${item.status}`} key={item.id}>
            <time>{new Date(item.starts_at).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}</time>
            <div className="card-body">
              <div className="card-title"><h3>{item.customer_name}</h3><span>{statusLabel[item.status]}</span></div>
              <p>{item.service_name} · {item.professional_name}</p>
              <b>{money.format(item.price_cents / 100)}</b>
              {item.status === 'scheduled' && <div className="actions"><button onClick={() => changeStatus(item, 'confirmed')}>Confirmar</button><button onClick={() => changeStatus(item, 'cancelled')}>Cancelar</button></div>}
              {item.status === 'confirmed' && <div className="actions"><button onClick={() => changeStatus(item, 'completed')}>Concluir</button><button onClick={() => changeStatus(item, 'cancelled')}>Cancelar</button></div>}
            </div>
          </article>)}</section>}
      </> : <NewAppointment services={services} professionals={professionals} customers={customers} onDone={async () => { setTab('agenda'); setOffset(0); await load() }} />}
    </main>

    <nav>
      <button className={tab === 'agenda' ? 'active' : ''} onClick={() => setTab('agenda')}><span>▦</span>Agenda</button>
      <button className={tab === 'new' ? 'active add' : 'add'} onClick={() => setTab('new')}><span>＋</span>Novo</button>
    </nav>
  </div>
}

function NewAppointment({ services, professionals, customers, onDone }: {
  services: Service[]; professionals: Professional[]; customers: Customer[]; onDone: () => Promise<void>
}) {
  const [form, setForm] = useState({ customer_id: '', professional_id: '', service_id: '', starts_at: '', notes: '' })
  const [name, setName] = useState(''); const [phone, setPhone] = useState('')
  const [list, setList] = useState(customers); const [saving, setSaving] = useState(false); const [error, setError] = useState('')
  async function addCustomer() {
    if (!name.trim()) return
    const customer = await api.createCustomer({ name, phone })
    setList(v => [...v, customer]); setForm(v => ({ ...v, customer_id: customer.id })); setName(''); setPhone('')
  }
  async function submit(event: FormEvent) {
    event.preventDefault(); setSaving(true); setError('')
    try { await api.createAppointment({ ...form, starts_at: new Date(form.starts_at).toISOString() }); await onDone() }
    catch (err) { setError(err instanceof Error ? err.message : 'Erro inesperado') }
    finally { setSaving(false) }
  }
  return <section className="form-page"><span className="eyebrow">NOVO HORÁRIO</span><h2>Agendar atendimento</h2>
    {error && <div className="alert">{error}</div>}
    <form onSubmit={submit}>
      <label>Cliente<select required value={form.customer_id} onChange={e => setForm({ ...form, customer_id: e.target.value })}><option value="">Selecione</option>{list.map(x => <option value={x.id} key={x.id}>{x.name}</option>)}</select></label>
      <details><summary>Cadastrar cliente rápido</summary><div className="quick"><input placeholder="Nome" value={name} onChange={e => setName(e.target.value)} /><input placeholder="Telefone" value={phone} onChange={e => setPhone(e.target.value)} /><button type="button" onClick={() => void addCustomer()}>Adicionar</button></div></details>
      <label>Serviço<select required value={form.service_id} onChange={e => setForm({ ...form, service_id: e.target.value })}><option value="">Selecione</option>{services.filter(x => x.active).map(x => <option value={x.id} key={x.id}>{x.name} · {money.format(x.price_cents / 100)}</option>)}</select></label>
      <label>Profissional<select required value={form.professional_id} onChange={e => setForm({ ...form, professional_id: e.target.value })}><option value="">Selecione</option>{professionals.filter(x => x.active).map(x => <option value={x.id} key={x.id}>{x.name}</option>)}</select></label>
      <label>Data e hora<input type="datetime-local" required value={form.starts_at} onChange={e => setForm({ ...form, starts_at: e.target.value })} /></label>
      <label>Observações<textarea rows={3} value={form.notes} onChange={e => setForm({ ...form, notes: e.target.value })} /></label>
      <button className="primary" disabled={saving}>{saving ? 'Salvando…' : 'Confirmar agendamento'}</button>
    </form>
  </section>
}
