import { describe, expect, it } from 'vitest'
import { isValidEmail, maskPhone, toE164BR } from './validation'

describe('maskPhone', () => {
  it('formats progressively as digits are typed', () => {
    expect(maskPhone('1')).toBe('(1')
    expect(maskPhone('11')).toBe('(11')
    expect(maskPhone('1198')).toBe('(11) 98')
    expect(maskPhone('119888')).toBe('(11) 9888')
    expect(maskPhone('1198887766')).toBe('(11) 9888-7766')
    expect(maskPhone('11988887766')).toBe('(11) 98888-7766')
  })

  it('strips non-digit characters and caps at 11 digits', () => {
    expect(maskPhone('(11) 98888-7766')).toBe('(11) 98888-7766')
    expect(maskPhone('11988887766999')).toBe('(11) 98888-7766')
  })

  it('returns an empty string for empty input', () => {
    expect(maskPhone('')).toBe('')
  })
})

describe('toE164BR', () => {
  it('prefixes +55 and strips formatting', () => {
    expect(toE164BR('(11) 98888-7766')).toBe('+5511988887766')
  })

  it('returns an empty string when there are no digits', () => {
    expect(toE164BR('')).toBe('')
    expect(toE164BR('()')).toBe('')
  })
})

describe('isValidEmail', () => {
  it('accepts well-formed addresses', () => {
    expect(isValidEmail('gestor@barberflow.local')).toBe(true)
    expect(isValidEmail('a.b+c@sub.example.com')).toBe(true)
  })

  it('rejects malformed addresses', () => {
    expect(isValidEmail('')).toBe(false)
    expect(isValidEmail('sem-arroba.com')).toBe(false)
    expect(isValidEmail('a@b')).toBe(false)
    expect(isValidEmail('a b@example.com')).toBe(false)
  })
})
