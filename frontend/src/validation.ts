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
