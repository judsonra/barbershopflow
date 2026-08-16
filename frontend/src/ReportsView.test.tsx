import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ReportsView } from './App'
import { api } from './api'
import type { Report } from './types'

vi.mock('./api', () => ({ api: { report: vi.fn() } }))

const mockedApi = vi.mocked(api, true)

beforeEach(() => { vi.clearAllMocks() })

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
  })

  it('shows the error message when the report fails to load', async () => {
    mockedApi.report.mockRejectedValue(new Error('falha ao carregar'))
    render(<ReportsView onBack={vi.fn()} />)

    expect(await screen.findByText('falha ao carregar')).toBeInTheDocument()
  })
})
