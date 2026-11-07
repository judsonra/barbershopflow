import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ProfessionalAccess } from './ProfessionalAccess'
import { ChangePassword } from './App'
import { api } from './api'
import type { Professional } from './types'

vi.mock('./api', () => ({
  ApiError: class ApiError extends Error {},
  api: { grantProfessionalAccess: vi.fn(), changePassword: vi.fn() },
}))

const mockedApi = vi.mocked(api, true)
const professional: Professional = { id: 'p1', name: 'Barbeiro', phone: '+5511988887777', active: true }

beforeEach(() => { vi.clearAllMocks() })

describe('ProfessionalAccess', () => {
  it('por padrão gera a senha para o gestor informar, com as instruções prontas', async () => {
    mockedApi.grantProfessionalAccess.mockResolvedValue({
      message: 'Senha gerada.', password: '482913', login: '+5511988887777', login_url: 'http://localhost:5173/junin',
    })
    render(<ProfessionalAccess professional={professional} onClose={vi.fn()} />)

    expect(screen.getByRole('radio', { name: 'Gerar senha para eu informar' })).toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: 'Gerar senha' }))

    await waitFor(() => expect(mockedApi.grantProfessionalAccess).toHaveBeenCalledWith('p1', { channel: 'manual', phone: '+5511988887777', email: undefined }))
    expect(await screen.findByText('482913')).toBeInTheDocument()
    const instructions = screen.getByText(/Seu acesso ao app da barbearia/)
    expect(instructions).toHaveTextContent('Endereço: http://localhost:5173/junin')
    expect(instructions).toHaveTextContent('Celular: (11) 98888-7777')
    expect(instructions).toHaveTextContent('Perfil > Alterar senha')
  })

  it('por e-mail exige um e-mail válido e envia', async () => {
    mockedApi.grantProfessionalAccess.mockResolvedValue({ message: 'Senha enviada para ana@example.com.' })
    render(<ProfessionalAccess professional={{ ...professional, phone: undefined }} onClose={vi.fn()} />)

    await userEvent.click(screen.getByRole('radio', { name: 'Enviar por e-mail' }))
    await userEvent.type(screen.getByLabelText('E-mail'), 'ana@exemplo')
    await userEvent.click(screen.getByRole('button', { name: 'Enviar senha' }))
    expect(screen.getByRole('alert')).toHaveTextContent('e-mail válido')
    expect(mockedApi.grantProfessionalAccess).not.toHaveBeenCalled()

    await userEvent.clear(screen.getByLabelText('E-mail'))
    await userEvent.type(screen.getByLabelText('E-mail'), 'ana@example.com')
    await userEvent.click(screen.getByRole('button', { name: 'Enviar senha' }))
    expect(await screen.findByText('Senha enviada para ana@example.com.')).toBeInTheDocument()
    expect(mockedApi.grantProfessionalAccess).toHaveBeenCalledWith('p1', { channel: 'email', phone: undefined, email: 'ana@example.com' })
  })

  it('manual sem celular nem e-mail não chama a API', async () => {
    render(<ProfessionalAccess professional={{ ...professional, phone: undefined }} onClose={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Gerar senha' }))
    expect(screen.getByRole('alert')).toHaveTextContent('celular ou o e-mail')
    expect(mockedApi.grantProfessionalAccess).not.toHaveBeenCalled()
  })
})

describe('ChangePassword (Perfil)', () => {
  async function fill(current: string, next: string, confirm: string) {
    await userEvent.click(screen.getByRole('button', { name: 'Alterar senha' }))
    await userEvent.type(screen.getByLabelText('Senha atual'), current)
    await userEvent.type(screen.getByLabelText('Nova senha'), next)
    await userEvent.type(screen.getByLabelText('Confirme a nova senha'), confirm)
    await userEvent.click(screen.getByRole('button', { name: 'Salvar nova senha' }))
  }

  it('troca a senha', async () => {
    mockedApi.changePassword.mockResolvedValue({ message: 'Senha alterada.' })
    render(<ChangePassword />)
    await fill('482913', 'minha-senha-nova', 'minha-senha-nova')
    expect(await screen.findByRole('status')).toHaveTextContent('Senha alterada.')
    expect(mockedApi.changePassword).toHaveBeenCalledWith('482913', 'minha-senha-nova')
  })

  it('confere confirmação e tamanho antes de enviar', async () => {
    render(<ChangePassword />)
    await fill('482913', 'minha-senha-nova', 'outra-coisa')
    expect(screen.getByRole('alert')).toHaveTextContent('confirmação não confere')
    expect(mockedApi.changePassword).not.toHaveBeenCalled()
  })
})
