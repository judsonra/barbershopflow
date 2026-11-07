import { render, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ReportsView } from './App'
import { api } from './api'
import type { Report } from './types'

vi.mock('./api', () => ({ ApiError: class ApiError extends Error {}, api: { report: vi.fn(), commissionReport: vi.fn() } }))

const mockedApi = vi.mocked(api, true)

const emptyCommissions = { from: '2026-01-01', to: '2026-02-01', total_commission_cents: 0, items: [] }

beforeEach(() => {
  vi.clearAllMocks()
  mockedApi.commissionReport.mockResolvedValue(emptyCommissions)
})

const report: Report = {
  from: '2026-01-01', to: '2026-02-01',
  occupancy: { overall_rate: 0.5, by_professional: [{ professional_id: 'p1', professional_name: 'Barbeiro', available_minutes: 100, booked_minutes: 50, rate: 0.5 }] },
  revenue: { total_cents: 10000, by_professional: [{ professional_id: 'p1', professional_name: 'Barbeiro', total_cents: 10000 }] },
  retention: { total_customers: 10, returning_customers: 4, rate: 0.4 },
}

describe('ReportsView', () => {
  it('loads and displays the report', async () => {
    mockedApi.report.mockResolvedValue(report)
    render(<ReportsView onBack={vi.fn()} />)

    expect(await screen.findByText('50%')).toBeInTheDocument()
    expect(screen.getAllByText('R$ 100,00')).toHaveLength(2)
    expect(screen.getByText(/4 de 10 clientes/)).toBeInTheDocument()
  })

  it('reloads when the date range changes', async () => {
    mockedApi.report.mockResolvedValue(report)
    render(<ReportsView onBack={vi.fn()} />)
    await waitFor(() => expect(mockedApi.report).toHaveBeenCalledTimes(1))
    expect(mockedApi.report.mock.calls[0][0]).toMatch(/^\d{4}-\d{2}-01$/)
  })

  it('shows the error message when the report fails to load', async () => {
    mockedApi.report.mockRejectedValue(new Error('falha ao carregar'))
    render(<ReportsView onBack={vi.fn()} />)

    expect(await screen.findByText('falha ao carregar')).toBeInTheDocument()
  })

  it('mostra as comissões por profissional no mesmo período', async () => {
    mockedApi.report.mockResolvedValue(report)
    mockedApi.commissionReport.mockResolvedValue({
      from: '2026-01-01', to: '2026-02-01', total_commission_cents: 4000,
      items: [{ professional_id: 'p1', professional_name: 'Barbeiro', appointments: 2, revenue_cents: 10000, commission_cents: 4000, without_rule: 1 }],
    })
    render(<ReportsView onBack={vi.fn()} />)

    const section = await screen.findByRole('region', { name: 'Comissões' })
    expect(await within(section).findByText('R$ 40,00')).toBeInTheDocument()
    expect(within(section).getByText(/2 atendimento\(s\)/)).toHaveTextContent('1 sem comissão cadastrada')
    expect(mockedApi.commissionReport).toHaveBeenCalledWith(mockedApi.report.mock.calls[0][0], mockedApi.report.mock.calls[0][1])
  })
})
