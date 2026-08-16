import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ProfessionalsManager } from './App'
import { api } from './api'
import type { Professional } from './types'

vi.mock('./api', () => ({
  api: {
    createProfessional: vi.fn(),
    updateProfessional: vi.fn(),
    getProfessionalServices: vi.fn(),
    setProfessionalServices: vi.fn(),
    grantProfessionalAccess: vi.fn(),
  },
}))

const mockedApi = vi.mocked(api, true)

beforeEach(() => {
  vi.clearAllMocks()
})

const existing: Professional = { id: 'p1', name: 'Barbeiro', active: true }
const validCPF = '111.444.777-35'

describe('ProfessionalsManager - create', () => {
  it('creates a professional with a valid e-mail and CPF', async () => {
    const created = { id: 'p2', name: 'Novo', active: true }
    mockedApi.createProfessional.mockResolvedValue(created)
    const onCreated = vi.fn()
    render(<ProfessionalsManager professionals={[]} services={[]} onCreated={onCreated} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Novo profissional' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'Novo')
    await userEvent.type(screen.getByPlaceholderText('E-mail (opcional)'), 'novo@x.com')
    await userEvent.type(screen.getByPlaceholderText('CPF (opcional)'), validCPF)
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar profissional' }))

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(created))
    expect(mockedApi.createProfessional).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Novo', email: 'novo@x.com', cpf: '11144477735' })
    )
  })

  it('rejects an invalid e-mail without calling the API', async () => {
    render(<ProfessionalsManager professionals={[]} services={[]} onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Novo profissional' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'Novo')
    await userEvent.type(screen.getByPlaceholderText('E-mail (opcional)'), 'not-an-email')
    fireEvent.submit(screen.getByPlaceholderText('Nome').closest('form')!)

    expect(await screen.findByText('Informe um e-mail válido ou deixe em branco.')).toBeInTheDocument()
    expect(mockedApi.createProfessional).not.toHaveBeenCalled()
  })

  it('rejects an invalid CPF without calling the API', async () => {
    render(<ProfessionalsManager professionals={[]} services={[]} onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Novo profissional' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'Novo')
    await userEvent.type(screen.getByPlaceholderText('CPF (opcional)'), '000.000.000-00')
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar profissional' }))

    expect(await screen.findByText('CPF inválido.')).toBeInTheDocument()
    expect(mockedApi.createProfessional).not.toHaveBeenCalled()
  })

  it('allows blank e-mail and CPF (both optional)', async () => {
    const created = { id: 'p3', name: 'SemContato', active: true }
    mockedApi.createProfessional.mockResolvedValue(created)
    render(<ProfessionalsManager professionals={[]} services={[]} onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Novo profissional' }))
    await userEvent.type(screen.getByPlaceholderText('Nome'), 'SemContato')
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar profissional' }))

    await waitFor(() => expect(mockedApi.createProfessional).toHaveBeenCalled())
  })
})

describe('ProfessionalsManager - specialties/pricing', () => {
  it('loads existing overrides and saves updated ones', async () => {
    mockedApi.getProfessionalServices.mockResolvedValue([{ service_id: 'svc-1', price_cents_override: 4000, duration_minutes_override: null }])
    mockedApi.setProfessionalServices.mockResolvedValue([])
    const services = [{ id: 'svc-1', name: 'Corte', duration_minutes: 30, price_cents: 5000, active: true }]
    render(<ProfessionalsManager professionals={[existing]} services={services} onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByText('Barbeiro'))
    await userEvent.click(screen.getByRole('button', { name: 'Especialidades e preços' }))

    const checkbox = await screen.findByRole('checkbox', { name: /Corte/ })
    await waitFor(() => expect(checkbox).toBeChecked())

    await userEvent.click(screen.getByRole('button', { name: 'Salvar especialidades' }))

    await waitFor(() => expect(mockedApi.setProfessionalServices).toHaveBeenCalledWith('p1', [
      { service_id: 'svc-1', price_cents_override: 4000, duration_minutes_override: null },
    ]))
  })
})

describe('ProfessionalsManager - grant access', () => {
  it('sends the access password by phone', async () => {
    mockedApi.grantProfessionalAccess.mockResolvedValue({ message: 'Senha enviada com sucesso.' })
    render(<ProfessionalsManager professionals={[existing]} services={[]} onCreated={vi.fn()} onUpdated={vi.fn()} onBack={vi.fn()} />)

    await userEvent.click(screen.getByText('Barbeiro'))
    await userEvent.click(screen.getByRole('button', { name: 'Conceder acesso' }))
    await userEvent.type(screen.getByPlaceholderText('(11) 99999-0000'), '11988887777')
    await userEvent.click(screen.getByRole('button', { name: 'Enviar senha' }))

    expect(await screen.findByText('Senha enviada com sucesso.')).toBeInTheDocument()
    expect(mockedApi.grantProfessionalAccess).toHaveBeenCalledWith('p1', '+5511988887777')
  })
})
