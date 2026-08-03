import { describe, expect, it } from 'vitest'
import { fromE164BR, isValidCPF, isValidEmail, maskCPF, maskPhone, toE164BR } from './validation'

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

describe('fromE164BR', () => {
  it('strips the +55 country code and re-masks', () => {
    expect(fromE164BR('+5511988887766')).toBe('(11) 98888-7766')
  })

  it('leaves an 11-digit number starting with DDD 55 alone', () => {
    expect(fromE164BR('55988887766')).toBe('(55) 98888-7766')
  })

  it('round-trips through toE164BR', () => {
    expect(fromE164BR(toE164BR('(21) 97777-6655'))).toBe('(21) 97777-6655')
  })
})

describe('maskCPF', () => {
  it('formats progressively as digits are typed', () => {
    expect(maskCPF('111')).toBe('111')
    expect(maskCPF('111444')).toBe('111.444')
    expect(maskCPF('111444777')).toBe('111.444.777')
    expect(maskCPF('11144477735')).toBe('111.444.777-35')
  })

  it('strips non-digit characters and caps at 11 digits', () => {
    expect(maskCPF('111.444.777-35')).toBe('111.444.777-35')
    expect(maskCPF('11144477735999')).toBe('111.444.777-35')
  })
})

describe('isValidCPF', () => {
  it('accepts a valid CPF, formatted or not', () => {
    expect(isValidCPF('111.444.777-35')).toBe(true)
    expect(isValidCPF('11144477735')).toBe(true)
  })

  it('rejects wrong check digits, repeated digits, and malformed input', () => {
    expect(isValidCPF('111.444.777-36')).toBe(false)
    expect(isValidCPF('111.111.111-11')).toBe(false)
    expect(isValidCPF('123456789')).toBe(false)
    expect(isValidCPF('')).toBe(false)
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
