import { describe, expect, it } from 'vitest'
import { buildICS, googleCalendarUrl } from './calendar'
import type { Appointment } from './types'

const appointment: Appointment = {
  id: 'abc-123',
  customer_id: 'cust-1',
  customer_name: 'Cliente Teste',
  professional_id: 'prof-1',
  professional_name: 'Rafael',
  service_id: 'svc-1',
  service_name: 'Corte',
  starts_at: '2026-08-10T14:00:00-03:00',
  ends_at: '2026-08-10T14:45:00-03:00',
  status: 'confirmed',
  notes: 'Preferência por máquina 1',
  price_cents: 5000
}

describe('buildICS', () => {
  it('produces a valid VCALENDAR with UTC dates and escaped text', () => {
    const ics = buildICS(appointment)
    expect(ics).toContain('BEGIN:VCALENDAR')
    expect(ics).toContain('END:VCALENDAR')
    expect(ics).toContain('UID:abc-123@barberflow')
    expect(ics).toContain('DTSTART:20260810T170000Z')
    expect(ics).toContain('DTEND:20260810T174500Z')
    expect(ics).toContain('SUMMARY:Corte · BarberFlow')
    expect(ics).toContain('Profissional: Rafael')
  })
})

describe('googleCalendarUrl', () => {
  it('builds a calendar.google.com quick-add URL with matching dates', () => {
    const url = googleCalendarUrl(appointment)
    expect(url.startsWith('https://calendar.google.com/calendar/render?')).toBe(true)
    const params = new URLSearchParams(url.split('?')[1])
    expect(params.get('action')).toBe('TEMPLATE')
    expect(params.get('dates')).toBe('20260810T170000Z/20260810T174500Z')
    expect(params.get('text')).toBe('Corte · BarberFlow')
  })
})
