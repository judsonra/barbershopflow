import { useEffect, useState } from 'react'
import { api, ApiError } from './api'
import type { CommissionReport } from './types'

const money = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })

function message(err: unknown) {
  return err instanceof ApiError || err instanceof Error ? err.message : 'Não foi possível carregar as comissões.'
}

// First day of this month and of the next, as AAAA-MM-DD in local time.
export function currentMonth() {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  const first = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-01`
  return { from: first(now), to: first(new Date(now.getFullYear(), now.getMonth() + 1, 1)) }
}

function useCommissionReport(from: string, to: string) {
  const [report, setReport] = useState<CommissionReport | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    setLoading(true); setError('')
    api.commissionReport(from, to)
      .then(result => { if (active) setReport(result) })
      .catch(err => { if (active) setError(message(err)) })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [from, to])
  return { report, loading, error }
}

// Relatórios > Comissões (gestor): commission per professional in the period.
export function CommissionsSection({ from, to }: { from: string; to: string }) {
  const { report, loading, error } = useCommissionReport(from, to)
  return <section className="commissions-report" aria-label="Comissões">
    <h3>Comissões</h3>
    {error && <div className="alert" role="alert">{error}</div>}
    {loading ? <p>Carregando…</p> : report && <>
      <p>Total a pagar: <b>{money.format(report.total_commission_cents / 100)}</b></p>
      {report.items.length === 0 ? <p>Nenhum atendimento concluído no período.</p> : <ul className="tenant-list">
        {report.items.map(item => <li key={item.professional_id}>
          <div>
            <b>{item.professional_name}</b>
            <small>
              {money.format(item.commission_cents / 100)} de comissão · {item.appointments} atendimento(s) · {money.format(item.revenue_cents / 100)} faturados
              {item.without_rule > 0 && ` · ${item.without_rule} sem comissão cadastrada`}
            </small>
          </div>
        </li>)}
      </ul>}
      <p className="hint">Calculado sobre atendimentos concluídos, com as regras de comissão atuais (Config → Profissionais → Comissão).</p>
    </>}
  </section>
}

// Aba "Comissões" do profissional: quanto ele tem a receber no período.
export function MyCommissions() {
  const month = currentMonth()
  const [from, setFrom] = useState(month.from)
  const [to, setTo] = useState(month.to)
  const { report, loading, error } = useCommissionReport(from, to)
  const mine = report?.items[0]

  return <section className="form-page"><span className="eyebrow">GANHOS</span><h2>Minhas comissões</h2>
    {error && <div className="alert" role="alert">{error}</div>}
    <div className="quick">
      <label>De<input type="date" lang="pt-BR" value={from} onChange={e => setFrom(e.target.value)} /></label>
      <label>Até<input type="date" lang="pt-BR" value={to} onChange={e => setTo(e.target.value)} /></label>
    </div>
    {loading ? <p>Carregando…</p> : report && <div className="profile-info">
      <div><small>A receber no período</small><b>{money.format(report.total_commission_cents / 100)}</b></div>
      <div><small>Atendimentos concluídos</small><b>{mine?.appointments ?? 0}</b></div>
      <div><small>Faturado nos seus atendimentos</small><b>{money.format((mine?.revenue_cents ?? 0) / 100)}</b></div>
      {mine && mine.without_rule > 0 && <div><small>Sem comissão cadastrada</small><b>{mine.without_rule}</b></div>}
    </div>}
    <p className="hint">Só entram atendimentos concluídos. Dúvidas sobre os percentuais, fale com o gestor.</p>
  </section>
}
