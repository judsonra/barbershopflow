import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MyCommissions, currentMonth } from './CommissionsReport'
import { CommissionsManager } from './CommissionsManager'
import { api } from './api'
import type { Service } from './types'

vi.mock('./api', () => ({
  ApiError: class ApiError extends Error {},
  api: { commissionReport: vi.fn(), getProfessionalCommissions: vi.fn(), setProfessionalCommissions: vi.fn() },
}))

const mockedApi = vi.mocked(api, true)
beforeEach(() => { vi.clearAllMocks() })

describe('MyCommissions (aba do profissional)', () => {
  it('mostra quanto o profissional tem a receber no mês', async () => {
    mockedApi.commissionReport.mockResolvedValue({
      from: '', to: '', total_commission_cents: 2000,
      items: [{ professional_id: 'p1', professional_name: 'Ana', appointments: 3, revenue_cents: 15000, commission_cents: 2000, without_rule: 0 }],
    })
    render(<MyCommissions />)

    expect(await screen.findByText('R$ 20,00')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('R$ 150,00')).toBeInTheDocument()
    const { from, to } = currentMonth()
    expect(mockedApi.commissionReport).toHaveBeenCalledWith(from, to)
  })

  it('sem atendimentos mostra zero', async () => {
    mockedApi.commissionReport.mockResolvedValue({ from: '', to: '', total_commission_cents: 0, items: [] })
    render(<MyCommissions />)
    expect(await screen.findAllByText('R$ 0,00', { selector: 'b' })).toHaveLength(2)
    expect(screen.getByText('0', { selector: 'b' })).toBeInTheDocument()
  })
})

describe('CommissionsManager (cadastro da regra)', () => {
  const services: Service[] = [
    { id: 's1', name: 'Corte', duration_minutes: 30, price_cents: 5000, active: true },
    { id: 's2', name: 'Barba', duration_minutes: 30, price_cents: 3000, active: true },
  ]

  it('carrega as regras, alterna para valor fixo e salva em centésimos/centavos', async () => {
    mockedApi.getProfessionalCommissions.mockResolvedValue([{ service_id: 's1', type: 'percentage', value: 4000 }])
    mockedApi.setProfessionalCommissions.mockResolvedValue([])
    render(<CommissionsManager id="p1" services={services} onClose={vi.fn()} />)

    expect(await screen.findByLabelText('Comissão de Corte')).toHaveValue('40,00')
    await userEvent.click(screen.getByRole('switch', { name: 'Comissão fixa para Barba' }))
    await userEvent.type(screen.getByLabelText('Comissão de Barba'), '12,50')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar comissões' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Comissões salvas.')
    expect(mockedApi.setProfessionalCommissions).toHaveBeenCalledWith('p1', [
      { service_id: 's1', type: 'percentage', value: 4000 },
      { service_id: 's2', type: 'fixed', value: 1250 },
    ])
  })

  it('recusa percentual acima de 100 sem chamar a API', async () => {
    mockedApi.getProfessionalCommissions.mockResolvedValue([])
    render(<CommissionsManager id="p1" services={services} onClose={vi.fn()} />)

    await userEvent.type(await screen.findByLabelText('Comissão de Corte'), '120')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar comissões' }))
    expect(screen.getByRole('alert')).toHaveTextContent('Comissão inválida para Corte')
    expect(mockedApi.setProfessionalCommissions).not.toHaveBeenCalled()
  })

  it('campo vazio remove a regra daquele serviço', async () => {
    mockedApi.getProfessionalCommissions.mockResolvedValue([{ service_id: 's1', type: 'fixed', value: 1000 }])
    mockedApi.setProfessionalCommissions.mockResolvedValue([])
    render(<CommissionsManager id="p1" services={services} onClose={vi.fn()} />)

    await userEvent.clear(await screen.findByLabelText('Comissão de Corte'))
    await userEvent.click(screen.getByRole('button', { name: 'Salvar comissões' }))
    expect(await screen.findByRole('status')).toBeInTheDocument()
    expect(mockedApi.setProfessionalCommissions).toHaveBeenCalledWith('p1', [])
  })
})
