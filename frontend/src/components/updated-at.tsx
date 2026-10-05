import { formatDateTime, timeAgo } from '@/lib/format'

export function UpdatedAt({ at }: { at: string }) {
  return <time dateTime={at} title={formatDateTime(at)} aria-label={`Last updated ${formatDateTime(at)}`} className="whitespace-nowrap tabular-nums">{timeAgo(at)}</time>
}
