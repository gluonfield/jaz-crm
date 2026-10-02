import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { FileText, MoreHorizontal, Plus, Trash2 } from 'lucide-react'
import { Fragment } from 'react'
import { Button } from '@jaz/ui/button'
import { Header, Tab } from '@/components/controls'
import { useNewPage } from '@/components/data-nav'
import { Field, TextInput } from '@/components/fields'
import { PageEditor } from '@/components/editor'
import { History } from '@/components/history'
import { ObjectIcon } from '@/components/icons'
import { PersonContext } from '@/components/person-context'
import { Related } from '@/components/related'
import { Summary } from '@/components/summary'
import { Timeline } from '@/components/timeline'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { useAction, useObjects, useTool, useUpcoming, useWrite } from '@/lib/queries'
import { path, usePages } from '@/lib/pages'
import type { CrmObject, CrmRecord, Ref } from '@/lib/types'

export const Route = createFileRoute('/_app/r/$recordId')({
  validateSearch: (search: Record<string, unknown>): { tab?: 'activity' } => (search.tab === 'activity' ? { tab: 'activity' } : {}),
  component: RecordPage,
})

function RecordPage() {
  const { recordId } = Route.useParams()
  const record = useTool<CrmRecord>('get_record', { record_id: recordId }).data
  const objects = useObjects()
  const object = objects?.find((o) => o.slug === record?.object)
  const upcoming = useUpcoming(recordId)
  const { tab } = Route.useSearch()
  const navigate = useNavigate()
  const remove = useAction<object>('delete_record')
  const pages = usePages()
  const newPage = useNewPage()
  if (!record || !objects || !object) {
    return <Header />
  }
  const name = recordName(record)
  const emails = valuesOf(record, 'email_addresses').map(valueText)
  const page = record.object === 'pages'
  const parent = (valuesOf(record, 'parent')[0] as Ref | undefined)?.id
  const header = (
    <Header>
      {page ? (
        <>
          <FileText />
          {pages &&
            path(pages, record.id)
              .slice(0, -1)
              .map((p) => (
                <Fragment key={p.id}>
                  <Link to="/r/$recordId" params={{ recordId: p.id }} className="max-w-48 truncate text-ink-2 hover:text-ink">
                    {p.name}
                  </Link>
                  <span className="text-ink-3">/</span>
                </Fragment>
              ))}
        </>
      ) : (
        <>
          <Link to="/o/$object" params={{ object: object.slug }} className="flex items-center gap-2 text-ink-2 hover:text-ink">
            <ObjectIcon slug={object.slug} className="size-4" />
            {object.name}
          </Link>
          <span className="text-ink-3">/</span>
        </>
      )}
      <span className="truncate">{name}</span>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" className="ml-auto" aria-label="More">
            <MoreHorizontal />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            onSelect={() =>
              remove.mutate(
                { record_id: record.id },
                { onSuccess: () => navigate(!page ? { to: '/o/$object', params: { object: object.slug } } : parent ? { to: '/r/$recordId', params: { recordId: parent } } : { to: '/' }) },
              )
            }
          >
            <Trash2 /> Delete {object.name.toLowerCase().replace(/s$/, '')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </Header>
  )
  if (page) {
    return (
      <>
        {header}
        <div className="scrollbar-quiet @container min-h-0 flex-1 overflow-y-auto">
          <div className="mx-auto flex max-w-[760px] flex-col gap-6 px-6 pb-24 pt-10">
            <h1 className="-ml-1.5 text-[22px] font-semibold leading-tight tracking-[-0.015em] text-ink">
              <Title key={`${record.id}:${name}`} record={record} object={object} name={name} />
            </h1>
            <PageEditor key={record.id} record={record} />
            <Related recordId={record.id} object={object.slug} objects={objects} />
            <Button variant="ghost" className="-ml-2 self-start" onClick={() => newPage(record.id)}>
              <Plus /> Add page
            </Button>
          </div>
        </div>
      </>
    )
  }
  return (
    <>
      {header}
      <div className="scrollbar-quiet @container min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto grid max-w-[1120px] gap-x-14 gap-y-8 px-6 pb-20 pt-8 [grid-template-areas:'summary'_'details'_'main'] @5xl:grid-cols-[minmax(0,1fr)_272px] @5xl:px-10 @5xl:[grid-template-areas:'summary_details'_'main_details']">
          <div className="min-w-0 [grid-area:summary]">
            <Summary record={record} object={object} name={name} upcoming={upcoming}>
              <h1 className="-ml-1.5 text-[22px] font-semibold leading-tight tracking-[-0.015em] text-ink">
                <Title key={`${record.id}:${name}`} record={record} object={object} name={name} />
              </h1>
            </Summary>
            {record.object === 'people' && <PersonContext key={`${record.id}:${record.values.context ?? ''}`} record={record} />}
            {object.attributes.some((a) => a.type === 'markdown') && (
              <div className="mt-6">
                <PageEditor key={record.id} record={record} />
              </div>
            )}
          </div>
          <Details record={record} object={object} />
          <div className="flex min-w-0 flex-col gap-8 [grid-area:main]">
            <Related recordId={record.id} object={object.slug} objects={objects} />
            <section>
              <div className="mb-4 flex gap-1.5 border-b border-border pb-2">
                <Tab active={!tab} onClick={() => void navigate({ to: '.', search: {}, replace: true })}>
                  Conversations
                </Tab>
                <Tab active={tab === 'activity'} onClick={() => void navigate({ to: '.', search: { tab: 'activity' }, replace: true })}>
                  Changes
                </Tab>
              </div>
              {tab ? (
                <History recordId={record.id} createdAt={record.created_at} attributes={object.attributes} />
              ) : (
                <Timeline recordId={record.id} object={object} people={emails} />
              )}
            </section>
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
  // A new page starts untitled, with its title ready to type.
  const untitled = record.object === 'pages' && name === 'Untitled'
  return (
    <TextInput label="Name" initial={untitled ? '' : name} placeholder={untitled ? 'Untitled' : 'Name'} autoFocus={untitled} required className="h-9 text-[22px] font-semibold" onCommit={(text) => write.set('name', text)} />
  )
}

// Details edits every attribute but the name, which the title edits. Beside
// the page on a wide screen, it stays in view as the timeline scrolls.
function Details({ record, object }: { record: CrmRecord; object: CrmObject }) {
  return (
    <section aria-label="Details" className="min-w-0 self-start [grid-area:details] @5xl:sticky @5xl:top-0">
      <h2 className="mb-1.5 text-[12px] font-medium text-ink-3">Details</h2>
      <dl className="grid grid-cols-[100px_minmax(0,1fr)] items-center gap-x-2 gap-y-0.5 text-[13px] @3xl:grid-cols-[100px_minmax(0,1fr)_100px_minmax(0,1fr)] @5xl:grid-cols-[100px_minmax(0,1fr)]">
        {object.attributes
          .filter((a) => a.slug !== 'name' && a.type !== 'markdown' && !(record.object === 'people' && a.slug === 'context') && !(record.object === 'follow_ups' && a.slug === 'draft'))
          .map((a) => (
            <div key={a.slug} className="contents">
              <dt className="truncate text-ink-3">{a.name}</dt>
              <dd className="min-w-0">
                <Field record={record} attribute={a} />
              </dd>
            </div>
          ))}
      </dl>
    </section>
  )
}
