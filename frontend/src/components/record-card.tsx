import { ArrowUpRight } from 'lucide-react'
import { RecordIcon } from '@/components/icons'
import { Stage } from '@/components/stage'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { formatNumber, recentOrDate } from '@/lib/format'
import { useTool } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'

const labels: Record<string, string> = { company: 'Company', people: 'Contacts', owner: 'Owner', job_title: 'Job title', context: 'Context', founded_year: 'Founded year', size: 'Size', domains: 'Website', email_addresses: 'Email', phone_numbers: 'Phone', categories: 'Categories', tags: 'Tags' }
const kinds: Record<string, string> = { people: 'Person', companies: 'Company', deals: 'Deal', pages: 'Page' }

export function RecordCard({ recordId, onOpen }: { recordId: string; onOpen: (url: string) => void }) {
  const result = useTool<CrmRecord>('get_record', { record_id: recordId }, { staleTime: 60_000 })
  if (result.isPending) {
    return <div className="h-24 animate-pulse rounded-xl bg-list-hover" aria-label="Loading record" />
  }
  if (result.isError) {
    return <p role="alert" className="m-0 px-3 py-2 text-danger">{result.error.message}</p>
  }
  const record = result.data
  const name = recordName(record)
  const stage = valuesOf(record, 'stage')[0]
  const value = valuesOf(record, 'value')[0]
  const description = valuesOf(record, 'description')[0]
  const fields = Object.keys(record.values).filter((slug) => !['name', 'stage', 'value', 'description'].includes(slug) && valuesOf(record, slug).length > 0)
  const href = `codex://plugins/jaz-crm/app/show_crm?path=${encodeURIComponent(`/r/${record.id}`)}`
  return (
    <a
      href={href}
      onClick={(event) => {
        event.preventDefault()
        onOpen(href)
      }}
      className="group block rounded-xl bg-raised p-4 text-ink outline-none transition-colors hover:bg-list-hover focus-visible:bg-list-hover"
    >
      <div className="flex items-start gap-3">
        <RecordIcon object={record.object} name={name} photo={record.photo} size={36} />
        <div className="min-w-0 flex-1">
          <h2 className="m-0 break-words text-[15px] font-semibold leading-5">{name}</h2>
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-ink-3">
            <span>{kinds[record.object] ?? record.object.replaceAll('_', ' ')}</span>
            {stage && <span className="text-ink-2"><Stage stage={valueText(stage)} /></span>}
          </div>
        </div>
        {value && <span className="shrink-0 text-[15px] font-medium tabular-nums">{formatNumber(valueText(value), 'value')}</span>}
        <ArrowUpRight aria-hidden className="mt-0.5 size-4 shrink-0 text-ink-3 group-hover:text-ink-2" />
      </div>
      {fields.length > 0 && (
        <dl className="mb-0 mt-3 grid grid-cols-[80px_minmax(0,1fr)] gap-x-3 gap-y-1.5 text-[12.5px] leading-5">
          {fields.map((slug) => (
            <div key={slug} className="contents">
              <dt className="truncate text-ink-3">{labels[slug] ?? slug.replaceAll('_', ' ')}</dt>
              <dd className={cn('m-0 min-w-0 whitespace-pre-wrap break-words text-ink-2', (slug === 'context' || slug === 'content') && 'line-clamp-3')}>{valuesOf(record, slug).map(valueText).join(', ')}</dd>
            </div>
          ))}
        </dl>
      )}
      {description && <p className="mb-0 mt-3 line-clamp-3 text-[12.5px] leading-5 text-ink-2">{valueText(description)}</p>}
      {record.activity?.last_at && (
        <p className="mb-0 mt-3 text-[12px] text-ink-3">
          Last contact {recentOrDate(record.activity.last_at)} · {record.activity.interactions} conversation{record.activity.interactions === 1 ? '' : 's'}
        </p>
      )}
    </a>
  )
}
