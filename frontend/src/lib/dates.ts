import { formatDay } from './format'

export function zonedInput(iso: string, timeZone: string) {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).formatToParts(new Date(iso))
  const get = (type: string) => parts.find((p) => p.type === type)?.value
  return `${get('year')}-${get('month')}-${get('day')}T${get('hour')}:${get('minute')}`
}

export function dateValue(day: string, time: string, zone: string, previous = '') {
  if (!day || !time) {
    return day
  }
  const local = `${day}T${time}`
  if (previous.length > 10 && zonedInput(previous, zone) === local) {
    return previous
  }
  const wall = Date.parse(`${local}:00Z`)
  // Try offsets on either side of a clock change. Reject a skipped local time.
  for (const delta of [-86400000, 0, 86400000]) {
    const sample = wall + delta
    const offset = Date.parse(`${zonedInput(new Date(sample).toISOString(), zone)}:00Z`) - sample
    const candidate = new Date(wall - offset).toISOString()
    if (zonedInput(candidate, zone) === local) {
      return candidate
    }
  }
  throw new Error('This time does not exist in the workspace timezone. Choose another time.')
}

export function formatActionDate(value: string, timeZone: string) {
  if (value.length === 10) {
    return formatDay(value)
  }
  const year = zonedInput(value, timeZone).slice(0, 4) === zonedInput(new Date().toISOString(), timeZone).slice(0, 4) ? undefined : 'numeric'
  const day = new Intl.DateTimeFormat('en', { timeZone, day: 'numeric', month: 'short', year }).format(new Date(value))
  const clock = new Intl.DateTimeFormat('en-GB', { timeZone, hour: '2-digit', minute: '2-digit' }).format(new Date(value))
  return `${day}, ${clock}`
}

// today is the date where the workspace is.
export const today = (zone: string, now = new Date()) => zonedInput(now.toISOString(), zone).slice(0, 10)

// isDue says whether a date has come, as the server's on_or_before now does:
// a day once it starts, a moment once it passes.
export function isDue(value: string, zone: string, now = new Date()) {
  return value.length === 10 ? value <= today(zone, now) : Date.parse(value) <= now.getTime()
}

// isOverdue says whether a date has gone by: a day once it ends.
export function isOverdue(value: string, zone: string, now = new Date()) {
  return value.length === 10 ? value < today(zone, now) : isDue(value, zone, now)
}

// dueLabel says in a word when a next step is due where the workspace is.
export function dueLabel(value: string, zone: string, now = new Date()) {
  const day = value.length === 10 ? value : zonedInput(value, zone).slice(0, 10)
  const days = Math.round((Date.parse(day) - Date.parse(today(zone, now))) / 86_400_000)
  if (isOverdue(value, zone, now)) {
    return { label: days < 0 ? `Overdue ${-days}d` : 'Overdue', late: true }
  }
  return { label: days === 0 ? 'Today' : days === 1 ? 'Tomorrow' : formatDay(day), late: false }
}
