import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { MoreHorizontal, Trash2 } from 'lucide-react'
import { Button, Header } from '@/components/controls'
import { Field } from '@/components/fields'
import { ObjectIcon, RecordIcon } from '@/components/icons'
import { Composer, InteractionRow } from '@/components/timeline'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { timeAgo } from '@/lib/format'
import { useAction, useObjects, useTimeline, useTool } from '@/lib/queries'
import type { Attribute, CrmRecord } from '@/lib/types'

export const Route = createFileRoute('/_app/r/$recordId')({ component: RecordPage })

function RecordPage() {
  const { recordId } = Route.useParams()
  const record = useTool<CrmRecord>('get_record', { record_id: recordId }).data
  const object = useObjects()?.find((o) => o.slug === record?.object)
  const navigate = useNavigate()
  const remove = useAction<object>('delete_record')
  if (!record || !object) {
    return <Header />
  }
  const name = recordName(record)
  const emails = valuesOf(record, 'email_addresses').map(valueText)
  return (
    <>
      <Header>
        <Link to="/o/$object" params={{ object: object.slug }} className="flex items-center gap-2 text-ink-2 hover:text-ink">
          <ObjectIcon slug={object.slug} className="size-4" />
          {object.name}
        </Link>
        <span className="text-ink-3">/</span>
        <span className="truncate">{name}</span>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button className="ml-auto border-transparent px-1.5" aria-label="More">
              <MoreHorizontal />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              onSelect={() => remove.mutate({ record_id: record.id }, { onSuccess: () => navigate({ to: '/o/$object', params: { object: object.slug } }) })}
            >
              <Trash2 /> Delete {object.name.toLowerCase().replace(/s$/, '')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </Header>
      <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto grid max-w-[1160px] gap-x-12 px-6 pb-16 pt-8 [grid-template-areas:'title'_'props'_'main'] xl:grid-cols-[minmax(0,1fr)_300px] xl:px-10 xl:[grid-template-areas:'title_props'_'main_props']">
          <div className="flex items-center gap-3 [grid-area:title]">
            <RecordIcon object={record.object} name={name} size={36} />
            <div className="min-w-0">
              <h1 className="truncate text-[20px] font-semibold tracking-[-0.01em] text-ink">{name}</h1>
              <p className="text-[12.5px] text-ink-3">
                {record.activity?.interactions
                  ? `${record.activity.interactions} conversation${record.activity.interactions === 1 ? '' : 's'} · last ${timeAgo(record.activity.last_at!)}`
                  : 'No conversations yet'}
              </p>
            </div>
          </div>
          <dl className="mt-6 grid grid-cols-[112px_minmax(0,1fr)] items-center gap-x-2 gap-y-1 self-start text-[13px] [grid-area:props] xl:sticky xl:top-0 xl:mt-1">
            {ordered(object.attributes).map((a) => (
              <div key={a.slug} className="contents">
                <dt className="truncate text-ink-3">{a.name}</dt>
                <dd className="min-w-0">
                  <Field record={record} attribute={a} />
                </dd>
              </div>
            ))}
          </dl>
          <div className="mt-6 min-w-0 [grid-area:main]">
            <Composer recordId={record.id} people={emails} />
            <Timeline recordId={record.id} />
          </div>
        </div>
      </div>
    </>
  )
}

// ordered puts a record's name first.
function ordered(attributes: Attribute[]) {
  return [...attributes].sort((a, b) => Number(b.slug === 'name') - Number(a.slug === 'name'))
}

function Timeline({ recordId }: { recordId: string }) {
  const timeline = useTimeline(recordId)
  const items = timeline.data?.pages.flatMap((p) => p.interactions) ?? []
  return (
    <>
      <ol className="-mx-3 mt-6 flex flex-col gap-0.5">
        {items.map((i) => (
          <InteractionRow key={i.id} interaction={i} />
        ))}
      </ol>
      {timeline.hasNextPage && (
        <Button className="mt-3" disabled={timeline.isFetchingNextPage} onClick={() => void timeline.fetchNextPage()}>
          Show earlier
        </Button>
      )}
    </>
  )
}
