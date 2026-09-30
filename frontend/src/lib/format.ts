const shortDate = new Intl.DateTimeFormat('en', { month: 'short', day: 'numeric' })
const longDate = new Intl.DateTimeFormat('en', { month: 'short', day: 'numeric', year: 'numeric' })
const clock = new Intl.DateTimeFormat('en', { hour: 'numeric', minute: '2-digit' })
const relative = new Intl.RelativeTimeFormat('en', { numeric: 'auto', style: 'narrow' })

export function formatDate(date: Date | string) {
  const parsed = new Date(date)
  return (parsed.getFullYear() === new Date().getFullYear() ? shortDate : longDate).format(parsed)
}

// formatDay shows a date attribute, stored as YYYY-MM-DD, in local time.
export function formatDay(day: string) {
  const [y, m, d] = day.split('-').map(Number)
  return formatDate(new Date(y, m - 1, d))
}

export function formatDateTime(iso: string) {
  return `${formatDate(iso)}, ${clock.format(new Date(iso))}`
}

export function timeAgo(iso: string) {
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

// localInput is a datetime-local input's value for a moment.
export function localInput(date: Date) {
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
