import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { BrandingConfig } from './BrandingConfig'
import { api } from './api'
import type { Tenant } from './types'

vi.mock('./api', () => ({
  ApiError: class ApiError extends Error {},
  api: { updateBranding: vi.fn(), uploadLogo: vi.fn(), deleteLogo: vi.fn() },
}))

const mockedApi = vi.mocked(api, true)
const tenant: Tenant = {
  id: 't1', name: 'Junin Barber Shop', slug: 'junin', self_scheduling_enabled: false, auto_confirm_appointments: false,
  cancellation_window_hours: 0, timezone: 'America/Sao_Paulo', active: true, created_at: '', color_scheme: 'classico',
}

beforeEach(() => {
  vi.clearAllMocks()
  URL.createObjectURL = vi.fn(() => 'blob:preview')
  URL.revokeObjectURL = vi.fn()
})

describe('BrandingConfig', () => {
  it('mostra o link da página da barbearia', () => {
    render(<BrandingConfig tenant={tenant} onSaved={vi.fn()} onBack={vi.fn()} />)
    expect(screen.getByRole('link', { name: `${window.location.origin}/junin` })).toBeInTheDocument()
  })

  it('troca a paleta com prévia na hora e salva', async () => {
    const saved = { ...tenant, color_scheme: 'vinho' }
    mockedApi.updateBranding.mockResolvedValue(saved)
    const onSaved = vi.fn()
    render(<BrandingConfig tenant={tenant} onSaved={onSaved} onBack={vi.fn()} />)

    const save = screen.getByRole('button', { name: 'Salvar cores' })
    expect(save).toBeDisabled()
    await userEvent.click(screen.getByRole('radio', { name: 'Vinho' }))
    expect(document.documentElement.dataset.colorScheme).toBe('vinho')
    await userEvent.click(save)

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(saved))
    expect(mockedApi.updateBranding).toHaveBeenCalledWith('vinho')
    expect(screen.getByRole('status')).toHaveTextContent('Cores salvas.')
  })

  it('sair sem salvar volta para a paleta salva', async () => {
    const { unmount } = render(<BrandingConfig tenant={tenant} onSaved={vi.fn()} onBack={vi.fn()} />)
    await userEvent.click(screen.getByRole('radio', { name: 'Oceano' }))
    expect(document.documentElement.dataset.colorScheme).toBe('oceano')
    unmount()
    expect(document.documentElement.dataset.colorScheme).toBe('classico')
  })

  it('envia a logo escolhida', async () => {
    const saved = { ...tenant, logo_url: '/api/v1/public/tenants/junin/logo?v=2' }
    mockedApi.uploadLogo.mockResolvedValue(saved)
    const onSaved = vi.fn()
    const { container } = render(<BrandingConfig tenant={tenant} onSaved={onSaved} onBack={vi.fn()} />)
    const file = new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], 'logo.png', { type: 'image/png' })

    await userEvent.upload(container.querySelector('input[type="file"]') as HTMLInputElement, file)
    expect(screen.getByRole('img', { name: 'Prévia da nova logo' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Enviar logo' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(saved))
    expect(mockedApi.uploadLogo).toHaveBeenCalledWith(file)
  })

  it('recusa arquivo grande ou em formato inválido antes de enviar', async () => {
    const { container } = render(<BrandingConfig tenant={tenant} onSaved={vi.fn()} onBack={vi.fn()} />)
    const input = container.querySelector('input[type="file"]') as HTMLInputElement

    await userEvent.upload(input, new File([new Uint8Array(1024 * 1024 + 1)], 'big.png', { type: 'image/png' }))
    expect(screen.getByRole('alert')).toHaveTextContent('no máximo 1 MB')

    await userEvent.upload(input, new File(['<svg/>'], 'logo.svg', { type: 'image/svg+xml' }), { applyAccept: false })
    expect(screen.getByRole('alert')).toHaveTextContent('PNG, JPG ou WebP')
    expect(mockedApi.uploadLogo).not.toHaveBeenCalled()
  })

  it('remove a logo atual', async () => {
    const withLogo = { ...tenant, logo_url: '/api/v1/public/tenants/junin/logo?v=1' }
    mockedApi.deleteLogo.mockResolvedValue(tenant)
    const onSaved = vi.fn()
    render(<BrandingConfig tenant={withLogo} onSaved={onSaved} onBack={vi.fn()} />)

    expect(screen.getByRole('img', { name: 'Logo da Junin Barber Shop' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Remover logo' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(tenant))
  })
})
