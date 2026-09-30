import { Link } from '@tanstack/react-router'
import { formatDateTime, timeAgo } from '@/lib/format'
import { useTool } from '@/lib/queries'
import type { Attribute } from '@/lib/types'

type Change = { attribute: string; value: string; record_id?: string; removed?: boolean; source?: string; actor?: string; at: string }

const sources: Record<string, string> = { sync: 'Sync', agent: 'An agent' }

// History lists a record's changes, newest first, ending with its creation.
export function History({ recordId, createdAt, attributes }: { recordId: string; createdAt: string; attributes: Attribute[] }) {
  const changes = useTool<{ changes: Change[] }>('record_history', { record_id: recordId }).data?.changes
  const label = (slug: string) => attributes.find((a) => a.slug === slug)?.name ?? slug
  return (
    <ol className="mt-4 flex flex-col text-[13px]">
      {changes?.map((c, index) => (
        <li key={index} className="flex items-baseline gap-2 py-1.5">
          <span className="min-w-0 flex-1 text-ink-2">
            {!c.removed && <span className="text-ink">{c.source === 'user' ? c.actor || 'Someone' : (sources[c.source ?? ''] ?? 'Someone')} </span>}
            {c.removed ? 'Removed ' : 'set '}
            <span className="text-ink">{label(c.attribute)}</span>
            {c.removed ? ' ' : ' to '}
            {c.record_id ? (
              <Link to="/r/$recordId" params={{ recordId: c.record_id }} className="text-ink hover:underline">
                {c.value}
              </Link>
            ) : (
              <span className="text-ink">{c.value}</span>
            )}
          </span>
          <time dateTime={c.at} title={formatDateTime(c.at)} className="shrink-0 text-[12px] text-ink-3">
            {timeAgo(c.at)}
          </time>
        </li>
      ))}
      {changes && (
        <li className="flex items-baseline gap-2 py-1.5 text-ink-2">
          <span className="flex-1">Created</span>
          <time dateTime={createdAt} title={formatDateTime(createdAt)} className="text-[12px] text-ink-3">
            {timeAgo(createdAt)}
          </time>
        </li>
      )}
    </ol>
  )
}
