import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ConfigMenu } from './App'

describe('ConfigMenu', () => {
  it('calls onSelect with the chosen view', async () => {
    const onSelect = vi.fn()
    render(<ConfigMenu onSelect={onSelect} />)

    await userEvent.click(screen.getByText('Serviços'))

    expect(onSelect).toHaveBeenCalledWith('services')
  })
})
