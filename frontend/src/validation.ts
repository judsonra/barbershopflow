// Brazilian phone mask: progressively formats digits as the user types,
// (DDD) NNNNN-NNNN for 9-digit mobile numbers or (DDD) NNNN-NNNN for
// 8-digit landlines. Always feed it back the previous masked value on
// change so backspacing over a formatting character behaves naturally.
export function maskPhone(value: string): string {
  const digits = value.replace(/\D/g, '').slice(0, 11)
  if (digits.length === 0) return ''
  if (digits.length <= 2) return `(${digits}`
  if (digits.length <= 6) return `(${digits.slice(0, 2)}) ${digits.slice(2)}`
  if (digits.length <= 10) return `(${digits.slice(0, 2)}) ${digits.slice(2, 6)}-${digits.slice(6)}`
  return `(${digits.slice(0, 2)}) ${digits.slice(2, 7)}-${digits.slice(7)}`
}

// Converts a masked/typed Brazilian phone into E.164 (+55DDDNNNNNNNNN)
// before it ever reaches the API — the format phone-login lookups match
// against exactly, and the one Zenvia needs to actually deliver an
// SMS/WhatsApp. Assumes Brazil, matching every other phone assumption
// already in this app (mask, placeholder, docs).
export function toE164BR(value: string): string {
  const digits = value.replace(/\D/g, '')
  return digits ? `+55${digits}` : ''
}

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function isValidEmail(value: string): boolean {
  return EMAIL_PATTERN.test(value)
}

// CPF mask: progressively formats as XXX.XXX.XXX-XX while typing.
export function maskCPF(value: string): string {
  const digits = value.replace(/\D/g, '').slice(0, 11)
  if (digits.length <= 3) return digits
  if (digits.length <= 6) return `${digits.slice(0, 3)}.${digits.slice(3)}`
  if (digits.length <= 9) return `${digits.slice(0, 3)}.${digits.slice(3, 6)}.${digits.slice(6)}`
  return `${digits.slice(0, 3)}.${digits.slice(3, 6)}.${digits.slice(6, 9)}-${digits.slice(9)}`
}

// Same check-digit algorithm as backend/internal/domain.ValidCPF — kept in
// sync so the UI can reject an invalid CPF before it ever reaches the API.
export function isValidCPF(value: string): boolean {
  const digits = value.replace(/\D/g, '')
  if (digits.length !== 11) return false
  if (/^(\d)\1{10}$/.test(digits)) return false
  const checkDigit = (length: number) => {
    let sum = 0
    for (let i = 0; i < length; i++) sum += Number(digits[i]) * (length + 1 - i)
    const remainder = (sum * 10) % 11
    return remainder === 10 ? 0 : remainder
  }
  return checkDigit(9) === Number(digits[9]) && checkDigit(10) === Number(digits[10])
}
