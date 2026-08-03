import type { Appointment } from './types'

// UTC "basic" format iCalendar/Google Calendar expect: YYYYMMDDTHHMMSSZ.
function toICSDate(iso: string) {
  return new Date(iso).toISOString().replace(/[-:]/g, '').split('.')[0] + 'Z'
}

function escapeICSText(value: string) {
  return value.replace(/\\/g, '\\\\').replace(/,/g, '\\,').replace(/;/g, '\\;').replace(/\n/g, '\\n')
}

function summary(appointment: Appointment) {
  return `${appointment.service_name} · BarberFlow`
}

function description(appointment: Appointment) {
  const lines = [`Profissional: ${appointment.professional_name}`]
  if (appointment.notes) lines.push(appointment.notes)
  return lines.join('\n')
}

export function buildICS(appointment: Appointment): string {
  return [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//BarberFlow//PT-BR',
    'BEGIN:VEVENT',
    `UID:${appointment.id}@barberflow`,
    `DTSTAMP:${toICSDate(new Date().toISOString())}`,
    `DTSTART:${toICSDate(appointment.starts_at)}`,
    `DTEND:${toICSDate(appointment.ends_at)}`,
    `SUMMARY:${escapeICSText(summary(appointment))}`,
    `DESCRIPTION:${escapeICSText(description(appointment))}`,
    'END:VEVENT',
    'END:VCALENDAR'
  ].join('\r\n')
}

// Triggers a browser download of the .ics file — the standard way to add an
// event to Apple Calendar, Outlook, and most Android calendar apps.
export function downloadICS(appointment: Appointment) {
  const blob = new Blob([buildICS(appointment)], { type: 'text/calendar;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `agendamento-${appointment.id}.ics`
  link.click()
  URL.revokeObjectURL(url)
}

// Opens Google Calendar's "quick add" web form pre-filled — the smoothest
// path on Android, where Calendar is usually already signed in.
export function googleCalendarUrl(appointment: Appointment): string {
  const params = new URLSearchParams({
    action: 'TEMPLATE',
    text: summary(appointment),
    dates: `${toICSDate(appointment.starts_at)}/${toICSDate(appointment.ends_at)}`,
    details: description(appointment)
  })
  return `https://calendar.google.com/calendar/render?${params.toString()}`
}
