import { useEffect, useState, FormEvent } from 'react'
import { api } from './api'
import type { ProfessionalCommission, Service } from './types'

export function CommissionsManager({ id, services, onClose }: { id: string; services: Service[]; onClose: () => void }) {
  const [entries, setEntries] = useState<Record<string, { type: 'percentage' | 'fixed'; value: string }>>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  useEffect(() => {
    let active = true
    api.getProfessionalCommissions(id).then(items => {
      if (active) setEntries(Object.fromEntries(items.map(item => [item.service_id, { type: item.type, value: (item.value / 100).toFixed(2).replace('.', ',') }])))
    }).catch(err => { if (active) setError(err.message) }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [id])
  async function submit(event: FormEvent) {
    event.preventDefault(); setSaved(false)
    const values: ProfessionalCommission[] = []
    for (const service of services) {
      const entry = entries[service.id]
      if (!entry || !entry.value.trim()) continue
      const value = Number(entry.value.replace(',', '.'))
      if (!Number.isFinite(value) || value < 0 || (entry.type === 'percentage' && value > 100)) {
        setError(`Comissão inválida para ${service.name}. Percentual deve estar entre 0 e 100.`); return
      }
      values.push({ service_id: service.id, type: entry.type, value: Math.round(value * 100) })
    }
    setSaving(true); setError('')
    try { await api.setProfessionalCommissions(id, values); setSaved(true) }
    catch (err) { setError(err instanceof Error ? err.message : 'Erro ao salvar') }
    finally { setSaving(false) }
  }
  function update(serviceId: string, patch: Partial<{ type: 'percentage' | 'fixed'; value: string }>) {
    setSaved(false)
    setEntries(old => ({ ...old, [serviceId]: { ...(old[serviceId] ?? { type: 'percentage', value: '' }), ...patch } }))
  }
  return <form className="quick" onSubmit={submit}>
    <h3>Comissão</h3>
    <p>Defina a comissão por serviço. Deixe em branco para remover a configuração.</p>
    {error && <div role="alert" className="alert">{error}</div>}
    {saved && <p role="status">Comissões salvas.</p>}
    {loading ? <p>Carregando…</p> : services.map(service => {
      const entry = entries[service.id] ?? { type: 'percentage', value: '' }
      return <fieldset key={service.id} disabled={saving}><legend>{service.name}</legend>
        <button type="button" role="switch" aria-checked={entry.type === 'fixed'} aria-label={`Comissão fixa para ${service.name}`} onClick={() => update(service.id, { type: entry.type === 'fixed' ? 'percentage' : 'fixed' })}>{entry.type === 'fixed' ? 'R$ · Fixo' : '% · Porcentagem'}</button>
        <label>Valor {entry.type === 'fixed' ? '(R$)' : '(%)'}<input aria-label={`Comissão de ${service.name}`} inputMode="decimal" value={entry.value} onChange={e => update(service.id, { value: e.target.value })} /></label>
      </fieldset>
    })}
    <button disabled={loading || saving}>{saving ? 'Salvando…' : 'Salvar comissões'}</button>
    <button type="button" disabled={saving} onClick={onClose}>Fechar</button>
  </form>
}
