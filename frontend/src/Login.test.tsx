import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { Login } from './App'
import { api, ApiError } from './api'

vi.mock('./api', () => {
  class ApiError extends Error {
    code?: string
    constructor(message: string, code?: string) {
      super(message)
      this.code = code
    }
  }
  return {
    ApiError,
    api: {
      login: vi.fn(),
      loginByPhone: vi.fn(),
      checkTenantName: vi.fn(),
      createTenant: vi.fn(),
      recoverPhone: vi.fn(),
      selectMembership: vi.fn(),
      socialLoginUrl: (provider: string) => `/api/v1/auth/${provider}/start`,
    },
  }
})

const mockedApi = vi.mocked(api, true)

beforeEach(() => {
  vi.clearAllMocks()
})

const baseUser = { id: 'u1', name: 'Fulano', role: 'manager' as const, active: true }

describe('Login - email mode', () => {
  it('logs in and calls onLogin on success', async () => {
    mockedApi.login.mockResolvedValue({ kind: 'ok', user: baseUser })
    const onLogin = vi.fn()
    render(<Login onLogin={onLogin} />)

    await userEvent.type(screen.getByLabelText('E-mail'), 'gestor@x.com')
    await userEvent.type(screen.getByLabelText('Senha'), 's3cret123')
    await userEvent.click(screen.getByRole('button', { name: 'Entrar' }))

    await waitFor(() => expect(onLogin).toHaveBeenCalledWith(baseUser))
    expect(mockedApi.login).toHaveBeenCalledWith('gestor@x.com', 's3cret123')
  })

  it('shows an error message when login fails', async () => {
    mockedApi.login.mockRejectedValue(new ApiError('e-mail ou senha inválidos'))
    render(<Login onLogin={vi.fn()} />)

    await userEvent.type(screen.getByLabelText('E-mail'), 'gestor@x.com')
    await userEvent.type(screen.getByLabelText('Senha'), 'wrong')
    await userEvent.click(screen.getByRole('button', { name: 'Entrar' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('e-mail ou senha inválidos')
  })
})

describe('Login - choose-tenant flow', () => {
  it('offers a membership choice and resolves it', async () => {
    mockedApi.login.mockResolvedValue({
      kind: 'choice', preauthToken: 'preauth-1',
      memberships: [
        { membership_id: 'm1', tenant_id: 't1', tenant_name: 'Barbearia A', tenant_slug: 'a', role: 'client' },
        { membership_id: 'm2', tenant_id: 't2', tenant_name: 'Barbearia B', tenant_slug: 'b', role: 'client' },
      ],
    })
    mockedApi.selectMembership.mockResolvedValue(baseUser)
    const onLogin = vi.fn()
    render(<Login onLogin={onLogin} />)

    await userEvent.type(screen.getByLabelText('E-mail'), 'cliente@x.com')
    await userEvent.type(screen.getByLabelText('Senha'), 's3cret123')
    await userEvent.click(screen.getByRole('button', { name: 'Entrar' }))

    expect(await screen.findByText('Barbearia A')).toBeInTheDocument()
    await userEvent.click(screen.getByText('Barbearia A'))

    await waitFor(() => expect(onLogin).toHaveBeenCalledWith(baseUser))
    expect(mockedApi.selectMembership).toHaveBeenCalledWith('preauth-1', 'm1')
  })
})

describe('Login - phone mode', () => {
  it('logs in by phone', async () => {
    mockedApi.loginByPhone.mockResolvedValue({ kind: 'ok', user: baseUser })
    const onLogin = vi.fn()
    render(<Login onLogin={onLogin} />)
    await userEvent.click(screen.getByRole('button', { name: 'Sou cliente e entro com celular' }))

    await userEvent.type(screen.getByLabelText('Celular'), '11999990000')
    await userEvent.type(screen.getByLabelText('Senha'), '123456')
    await userEvent.click(screen.getByRole('button', { name: 'Entrar' }))

    await waitFor(() => expect(onLogin).toHaveBeenCalledWith(baseUser))
  })

  it('redirects to recover mode on account_locked', async () => {
    mockedApi.loginByPhone.mockRejectedValue(new ApiError('conta bloqueada', 'account_locked'))
    render(<Login onLogin={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Sou cliente e entro com celular' }))

    await userEvent.type(screen.getByLabelText('Celular'), '11999990000')
    await userEvent.type(screen.getByLabelText('Senha'), 'wrong')
    await userEvent.click(screen.getByRole('button', { name: 'Entrar' }))

    expect(await screen.findByRole('button', { name: 'Receber nova senha por SMS/WhatsApp' })).toBeInTheDocument()
  })
})

describe('Login - recover mode', () => {
  it('shows the server message after requesting a new password', async () => {
    mockedApi.recoverPhone.mockResolvedValue({ message: 'Se o número estiver cadastrado, uma nova senha foi enviada.' })
    render(<Login onLogin={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Sou cliente e entro com celular' }))
    await userEvent.click(screen.getByRole('button', { name: 'Esqueci minha senha' }))

    await userEvent.type(screen.getByLabelText('Celular cadastrado'), '11999990000')
    await userEvent.click(screen.getByRole('button', { name: 'Receber nova senha por SMS/WhatsApp' }))

    expect(await screen.findByText('Se o número estiver cadastrado, uma nova senha foi enviada.')).toBeInTheDocument()
  })
})

describe('Login - signup mode', () => {
  async function goToSignup() {
    render(<Login onLogin={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Cadastrar minha barbearia' }))
  }

  it('rejects a short password client-side, without calling the API', async () => {
    await goToSignup()
    await userEvent.type(screen.getByLabelText('Nome da barbearia'), 'Barbearia Nova')
    await userEvent.type(screen.getByLabelText('Seu nome'), 'Fulano')
    await userEvent.type(screen.getByLabelText('E-mail'), 'fulano@x.com')
    // Below the 8-char minLength: some mobile browsers swallow the native
    // validation bubble, so submitSignup double-checks in JS (see App.tsx) —
    // that's the behavior under test here, not jsdom's own constraint
    // validation (unreliable in a DOM without real layout/interaction).
    const passwordInput = screen.getByLabelText('Senha')
    await userEvent.type(passwordInput, 'short')
    passwordInput.removeAttribute('minlength')
    await userEvent.click(screen.getByRole('button', { name: 'Criar barbearia' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('A senha deve ter pelo menos 8 caracteres.')
    expect(mockedApi.createTenant).not.toHaveBeenCalled()
  })

  it('creates the tenant and logs in on success', async () => {
    mockedApi.createTenant.mockResolvedValue(baseUser)
    const onLogin = vi.fn()
    render(<Login onLogin={onLogin} />)
    await userEvent.click(screen.getByRole('button', { name: 'Cadastrar minha barbearia' }))

    await userEvent.type(screen.getByLabelText('Nome da barbearia'), 'Barbearia Nova')
    await userEvent.type(screen.getByLabelText('Seu nome'), 'Fulano')
    await userEvent.type(screen.getByLabelText('E-mail'), 'fulano@x.com')
    await userEvent.type(screen.getByLabelText('Senha'), 's3cret123')
    await userEvent.click(screen.getByRole('button', { name: 'Criar barbearia' }))

    await waitFor(() => expect(onLogin).toHaveBeenCalledWith(baseUser))
    expect(mockedApi.createTenant).toHaveBeenCalledWith(expect.objectContaining({
      tenant_name: 'Barbearia Nova', manager_name: 'Fulano', email: 'fulano@x.com', password: 's3cret123',
    }))
  })
})
