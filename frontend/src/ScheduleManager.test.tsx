import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ScheduleManager } from './App'
import { api } from './api'
import type { Professional, User } from './types'

vi.mock('./api', () => ({
  api: {
    getSchedule: vi.fn(),
    setSchedule: vi.fn(),
    listTimeOff: vi.fn(),
    createTimeOff: vi.fn(),
    deleteTimeOff: vi.fn(),
  },
}))

const mockedApi = vi.mocked(api, true)

beforeEach(() => {
  vi.clearAllMocks()
  mockedApi.getSchedule.mockResolvedValue([])
  mockedApi.listTimeOff.mockResolvedValue([])
})

const professionals: Professional[] = [{ id: 'p1', name: 'Barbeiro A', active: true }, { id: 'p2', name: 'Barbeiro B', active: true }]
const manager: User = { id: 'm1', name: 'Gestor', role: 'manager', active: true }
const professionalUser: User = { id: 'u1', name: 'Barbeiro A', role: 'professional', professional_id: 'p1', active: true }

describe('ScheduleManager', () => {
  it('lets a manager pick any professional', async () => {
    render(<ScheduleManager user={manager} professionals={professionals} />)
    await waitFor(() => expect(mockedApi.getSchedule).toHaveBeenCalledWith('p1'))
    expect(screen.getByLabelText('Profissional')).toBeInTheDocument()
  })

  it('locks a professional to their own schedule (no selector)', async () => {
    render(<ScheduleManager user={professionalUser} professionals={professionals} />)
    await waitFor(() => expect(mockedApi.getSchedule).toHaveBeenCalledWith('p1'))
    expect(screen.queryByLabelText('Profissional')).not.toBeInTheDocument()
  })

  it('saves the weekly schedule', async () => {
    mockedApi.setSchedule.mockResolvedValue([])
    render(<ScheduleManager user={manager} professionals={professionals} />)
    await waitFor(() => expect(mockedApi.getSchedule).toHaveBeenCalled())

    await userEvent.click(screen.getByLabelText('Segunda'))
    await userEvent.click(screen.getByRole('button', { name: 'Salvar jornada' }))

    await waitFor(() => expect(mockedApi.setSchedule).toHaveBeenCalledWith('p1', [{ weekday: 1, start_minute: 540, end_minute: 1080 }]))
    expect(await screen.findByText('Jornada salva.')).toBeInTheDocument()
  })

  it('adds a time-off block', async () => {
    const created = { id: 'b1', professional_id: 'p1', starts_at: '2026-06-01T09:00:00.000Z', ends_at: '2026-06-01T10:00:00.000Z', reason: 'Viagem' }
    mockedApi.createTimeOff.mockResolvedValue(created)
    render(<ScheduleManager user={manager} professionals={professionals} />)
    await waitFor(() => expect(mockedApi.getSchedule).toHaveBeenCalled())

    await userEvent.type(screen.getByLabelText('Início'), '2026-06-01T09:00')
    await userEvent.type(screen.getByLabelText('Fim'), '2026-06-01T10:00')
    await userEvent.type(screen.getByPlaceholderText('Motivo (folga, viagem, ausência...)'), 'Viagem')
    await userEvent.click(screen.getByRole('button', { name: 'Bloquear período' }))

    await waitFor(() => expect(mockedApi.createTimeOff).toHaveBeenCalledWith('p1', expect.objectContaining({ reason: 'Viagem' })))
  })
})
