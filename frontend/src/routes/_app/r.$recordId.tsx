import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { MoreHorizontal, Trash2 } from 'lucide-react'
import { Button, Header, Tab } from '@/components/controls'
import { Field, TextInput } from '@/components/fields'
import { History } from '@/components/history'
import { ObjectIcon, RecordIcon } from '@/components/icons'
import { Related } from '@/components/related'
import { Composer, InteractionRow } from '@/components/timeline'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { timeAgo } from '@/lib/format'
import { useAction, useObjects, useTimeline, useTool, useWrite } from '@/lib/queries'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'

export const Route = createFileRoute('/_app/r/$recordId')({
  validateSearch: (search: Record<string, unknown>): { tab?: 'activity' } => (search.tab === 'activity' ? { tab: 'activity' } : {}),
  component: RecordPage,
})

function RecordPage() {
  const { recordId } = Route.useParams()
  const record = useTool<CrmRecord>('get_record', { record_id: recordId }).data
  const objects = useObjects()
  const object = objects?.find((o) => o.slug === record?.object)
  const { tab } = Route.useSearch()
  const navigate = useNavigate()
  const remove = useAction<object>('delete_record')
  if (!record || !objects || !object) {
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
            <RecordIcon object={record.object} name={name} photo={record.photo} size={36} />
            <div className="min-w-0">
              <h1 className="-ml-1.5 text-[20px] font-semibold tracking-[-0.01em] text-ink">
                <Title key={`${record.id}:${name}`} record={record} object={object} name={name} />
              </h1>
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
          <div className="min-w-0 [grid-area:main]">
            <Related recordId={record.id} object={object.slug} objects={objects} />
            <div className="mt-8 flex gap-1 border-b border-border pb-2">
              <Tab active={!tab} onClick={() => void navigate({ to: '.', search: {}, replace: true })}>
                Conversations
              </Tab>
              <Tab active={tab === 'activity'} onClick={() => void navigate({ to: '.', search: { tab: 'activity' }, replace: true })}>
                Activity
              </Tab>
            </div>
            {!tab ? (
              <div className="mt-4">
                <Composer recordId={record.id} people={emails} />
                <Timeline recordId={record.id} object={object.slug} />
              </div>
            ) : (
              <History recordId={record.id} createdAt={record.created_at} attributes={object.attributes} />
            )}
          </div>
        </div>
      </div>
    </>
  )
}

// Title shows a record's name, which a click edits when the object has one.
function Title({ record, object, name }: { record: CrmRecord; object: CrmObject; name: string }) {
  const write = useWrite(record)
  if (!object.attributes.some((a) => a.slug === 'name')) {
    return <span className="block truncate px-1.5">{name}</span>
  }
  return <TextInput label="Name" initial={name} placeholder="Name" required className="h-9 text-[20px]" onCommit={(text) => write.set('name', text)} />
}

// ordered puts a record's name first.
function ordered(attributes: Attribute[]) {
  return [...attributes].sort((a, b) => Number(b.slug === 'name') - Number(a.slug === 'name'))
}

// arrivals says what fills a record's timeline.
const arrivals: Record<string, string> = {
  people: 'Emails and meetings with this person appear here as they sync, along with the calls and notes you log above.',
  companies: 'Emails and meetings with people at this company appear here as they sync, along with the calls and notes you log above.',
}

function Timeline({ recordId, object }: { recordId: string; object: string }) {
  const timeline = useTimeline(recordId)
  const items = timeline.data?.pages.flatMap((p) => p.interactions) ?? []
  return (
    <>
      {timeline.data && items.length === 0 && (
        <p className="mt-6 text-[13px] leading-[1.5] text-ink-3">
          {arrivals[object] ?? 'Conversations you link to this record appear here, along with the calls and notes you log above.'}
        </p>
      )}
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
