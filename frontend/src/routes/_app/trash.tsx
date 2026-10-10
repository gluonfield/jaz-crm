import { createFileRoute } from '@tanstack/react-router'
import { Trash2 } from 'lucide-react'
import { Button } from '@jaz/ui/button'
import { Header } from '@/components/controls'
import { NavButton } from '@/components/nav-drawer'
import { ListMotion } from '@/components/list-motion'
import { ObjectIcon } from '@/components/icons'
import { PageIcon } from '@/components/page-icon'
import { formatDate, timeAgo } from '@/lib/format'
import { useAction, useTool } from '@/lib/queries'
import type { TrashedRecord } from '@/lib/types'

export const Route = createFileRoute('/_app/trash')({ component: TrashPage })

function TrashPage() {
  const trash = useTool<{ records: TrashedRecord[] }>('list_trash')
  return (
    <>
      <Header><NavButton /><Trash2 /> Trash</Header>
      <ListMotion className="flex min-h-0 flex-1 flex-col">
        <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-[760px] px-6 py-8">
            {trash.isError && <p className="text-[13px] text-danger">Couldn’t load Trash.</p>}
            {trash.data?.records.length === 0 && <p className="text-[13px] text-ink-3">Trash is empty.</p>}
            {trash.data?.records.map((record) => <TrashedRow key={record.id} record={record} />)}
          </div>
        </div>
      </ListMotion>
    </>
  )
}

function TrashedRow({ record }: { record: TrashedRecord }) {
  const restore = useAction<{ record_id: string }>('restore_record')
  return (
    <div data-flip={record.id} className="flex min-h-16 items-center gap-3 border-b border-border-soft py-3 text-[13px]">
      {record.object === 'pages' ? <PageIcon value={record.icon} /> : <ObjectIcon slug={record.object} className="size-4 shrink-0 text-ink-3" />}
      <div className="min-w-0 flex-1">
        <div className="truncate font-medium text-ink" title={record.name}>{record.name}</div>
        <div className="mt-0.5 text-[12px] text-ink-3" title={formatDate(record.deleted_at)}>Deleted {timeAgo(record.deleted_at)}</div>
      </div>
      <Button className="min-h-10" disabled={restore.isPending} aria-label={`Restore ${record.name}`} onClick={() => restore.mutate({ record_id: record.id })}>
        Restore
      </Button>
    </div>
  )
}
