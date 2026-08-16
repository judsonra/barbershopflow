import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Profile } from './App'
import type { User } from './types'

const user: User = { id: 'u1', name: 'Fulano', email: 'fulano@x.com', role: 'manager', active: true }

describe('Profile', () => {
  it('displays the user data', () => {
    render(<Profile user={user} onLogout={vi.fn()} onBack={vi.fn()} />)
    expect(screen.getByText('Fulano')).toBeInTheDocument()
    expect(screen.getByText('fulano@x.com')).toBeInTheDocument()
    expect(screen.getByText('Gestor')).toBeInTheDocument()
  })

  it('calls onLogout when Sair is clicked', async () => {
    const onLogout = vi.fn()
    render(<Profile user={user} onLogout={onLogout} onBack={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Sair' }))
    expect(onLogout).toHaveBeenCalled()
  })
})
