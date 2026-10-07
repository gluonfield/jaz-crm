import { RecordIcon } from '@/components/icons'
import { Stage } from '@/components/stage'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { pageIcon } from '@/lib/pages'
import { formatNumber } from '@/lib/format'
import { useTool } from '@/lib/queries'
import { recordSearchInput } from '@/lib/record-search'
import type { CrmRecord } from '@/lib/types'

export function RecordResults({ path, onOpen }: { path: string; onOpen: (url: string) => void }) {
  const input = recordSearchInput(path)
  const result = useTool<{ records: CrmRecord[] }>('search_records', input, { enabled: !!input.object, staleTime: 60_000 })
  if (!input.object) {
    return null
  }
  if (result.isPending) {
    return <div className="h-10 animate-pulse bg-list-hover" aria-label="Loading records" />
  }
  if (result.isError) {
    return <p role="alert" className="m-0 px-3 py-2 text-danger">{result.error.message}</p>
  }
  if (result.data.records.length === 0) {
    return <p className="m-0 px-3 py-2 text-ink-3">No matching records</p>
  }
  return (
    <ul className="m-0 list-none p-0">
      {result.data.records.map((record) => {
        const name = recordName(record)
        const detail = valuesOf(record, 'company')[0] ?? valuesOf(record, 'domains')[0] ?? valuesOf(record, 'emails')[0]
        const stage = valuesOf(record, 'stage')[0]
        const value = valuesOf(record, 'value')[0]
        const href = `codex://plugins/jaz-crm/app/show_crm?path=${encodeURIComponent(`/r/${record.id}`)}`
        return (
          <li key={record.id} className="border-b border-border/50 last:border-0">
            <a href={href} onClick={(event) => {
              event.preventDefault()
              onOpen(href)
            }} className="flex min-h-10 items-center gap-3 px-3 py-2 text-[13px] text-ink outline-none hover:bg-list-hover focus-visible:bg-list-hover">
              <RecordIcon object={record.object} name={name} photo={record.photo} icon={pageIcon(record)} />
              <span className="min-w-0 flex-1 truncate font-medium">{name}</span>
              {detail && <span className="hidden max-w-40 truncate text-ink-3 sm:block">{valueText(detail)}</span>}
              {stage && <span className="max-w-28 truncate text-[12px] text-ink-2"><Stage stage={valueText(stage)} /></span>}
              {value && <span className="shrink-0 tabular-nums text-ink-2">{formatNumber(valueText(value), 'value')}</span>}
            </a>
          </li>
        )
      })}
    </ul>
  )
}
