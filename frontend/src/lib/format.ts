const shortDate = new Intl.DateTimeFormat('en', { month: 'short', day: 'numeric' })
const longDate = new Intl.DateTimeFormat('en', { month: 'short', day: 'numeric', year: 'numeric' })
const clock = new Intl.DateTimeFormat('en', { hour: 'numeric', minute: '2-digit' })
const relative = new Intl.RelativeTimeFormat('en', { numeric: 'auto', style: 'narrow' })

export function formatNumber(value: number | string, slug?: string, compact = false) {
  return new Intl.NumberFormat('en', {
    style: slug === 'value' ? 'currency' : 'decimal',
    currency: 'USD',
    useGrouping: slug !== 'founded_year',
    minimumFractionDigits: 0,
    maximumFractionDigits: compact ? 1 : 2,
    notation: compact ? 'compact' : 'standard',
  }).format(Number(value))
}

export function formatDate(date: Date | string): string {
  if (typeof date === 'string' && date.length === 10) {
    return formatDay(date)
  }
  const parsed = new Date(date)
  return (parsed.getFullYear() === new Date().getFullYear() ? shortDate : longDate).format(parsed)
}

// formatDay shows a date attribute, stored as YYYY-MM-DD, in local time.
export function formatDay(day: string): string {
  const [y, m, d] = day.split('-').map(Number)
  return formatDate(new Date(y, m - 1, d))
}

export function formatDateTime(iso: string) {
  return iso.length === 10 ? formatDay(iso) : `${formatDate(iso)}, ${formatTime(iso)}`
}

export const formatTime = (iso: string) => clock.format(new Date(iso))

export function timeAgo(iso: string) {
  if (iso.length === 10) {
    return formatDay(iso)
  }
  const seconds = (new Date(iso).getTime() - Date.now()) / 1000
  const units: [Intl.RelativeTimeFormatUnit, number][] = [
    ['year', 31536000],
    ['month', 2592000],
    ['week', 604800],
    ['day', 86400],
    ['hour', 3600],
    ['minute', 60],
  ]
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size) {
      return relative.format(Math.round(seconds / size), unit)
    }
  }
  return 'just now'
}

// recentOrDate says how long ago a moment of the last week was, and the date
// of an older one.
export function recentOrDate(iso: string) {
  if (iso.length === 10) {
    return formatDay(iso)
  }
  return Date.now() - new Date(iso).getTime() < 7 * 86400000 ? timeAgo(iso) : formatDate(iso)
}

export const hasEnded = (iso: string) => new Date(iso).getTime() < Date.now()

const weekday = new Intl.DateTimeFormat('en', { weekday: 'short' })

// meetingTime says when a meeting runs: its day, its start and end, and how
// long it lasts.
export function meetingTime(start: string, end?: string) {
  if (start.length === 10) {
    return formatDay(start)
  }
  const from = new Date(start)
  const text = `${weekday.format(from)}, ${formatDate(from)} · ${clock.format(from)}`
  if (!end) {
    return text
  }
  const to = new Date(end)
  const minutes = Math.round((to.getTime() - from.getTime()) / 60000)
  const length = minutes < 60 ? `${minutes} min` : `${Math.floor(minutes / 60)} h${minutes % 60 ? ` ${minutes % 60} min` : ''}`
  return `${text} – ${clock.format(to)} · ${length}`
}

// localInput is a datetime-local input's value for a moment.
export function localInput(date: Date) {
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
