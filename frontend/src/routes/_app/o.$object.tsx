import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { ArrowDownAZ, ArrowUp, Kanban, ListChecks, Plus, Search, Table2, X } from 'lucide-react'
import { type ReactNode, useMemo, useRef, useState } from 'react'
import { Board } from '@/components/board'
import { CompanyPeople } from '@/components/company-people'
import { editDraft } from '@/components/follow-up'
import { byUrgency, FollowUpQueue } from '@/components/follow-ups'
import { Kbd } from '@/components/kbd'
import { Stage } from '@/components/stage'
import { Button } from '@jaz/ui/button'
import { Header, Loading } from '@/components/controls'
import { AddColumn, ColumnHeader } from '@/components/columns'
import { CreateRecord } from '@/components/create-record'
import { Field } from '@/components/fields'
import { ExternalLink } from '@/components/external-link'
import { ConnectGoogle, EmptyState } from '@/components/empty-state'
import { ObjectIcon, RecordIcon } from '@/components/icons'
import { Picker } from '@/components/picker'
import { OwnerFilter } from '@/components/owner-filter'
import { RecordFilters } from '@/components/record-filters'
import { RecordMenu } from '@/components/record-menu'
import { SelectField } from '@/components/select-field'
import { UpdatedAt } from '@/components/updated-at'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { pageIcon } from '@/lib/pages'
import { formatDay, formatNumber } from '@/lib/format'
import { useDebounced, useFlip, useInView, useListKeys } from '@/lib/hooks'
import { DateLabel, dateMetadata } from '@/components/date-field'
import { useObjects, useRecordPages, useTool, useWorkspace } from '@/lib/queries'
import { useColumnWidths } from '@/lib/use-column-widths'
import { validateRecordSearch } from '@/lib/record-search'
import { statusOf } from '@/lib/stages'
import { useMail } from '@/lib/sync'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_app/o/$object')({
  validateSearch: validateRecordSearch,
  component: ObjectPage,
})

// arrivals says how synced objects' records arrive.
const arrivals: Record<string, string> = {
  people: 'People you write to or meet appear here automatically, along with anyone you keep in Triage.',
  companies: 'Companies appear here from the work email domains of the people you keep.',
  follow_ups: 'Next steps appear here as conversations with the people you keep create them.',
}

function ObjectPage() {
  const { object: slug } = Route.useParams()
  const workspace = useWorkspace()
  return workspace && <ObjectList key={`${workspace.id}:${slug}`} slug={slug} />
}

function ObjectList({ slug }: { slug: string }) {
  const objects = useObjects()
  const object = objects?.find((o) => o.slug === slug)
  const me = useWorkspace()?.members?.find((m) => m.is_me)
  const search = Route.useSearch()
  const { sort, view, q = '', filters = slug === 'follow_ups' ? [
    { attribute: 'status', operator: 'is', value: 'Open' },
    ...(me ? [{ attribute: 'owner', operator: 'is' as const, value: me.email }] : []),
  ] : [], saved, limit = 100, conversation_id } = search
  const query = useDebounced(q.trim())
  const queue = slug === 'follow_ups' && (view ? view !== 'table' : search.group_by_conversation !== false)
  const scope = { group_by_conversation: slug === 'follow_ups' ? queue : undefined, conversation_id }
  const status = object && statusOf(object.attributes)
  // The workspace's own tables open as tables, the CRM's pipelines as boards.
  const board = slug !== 'follow_ups' && (view ?? (object?.standard ? 'board' : 'table')) === 'board' ? status : undefined
  const everything = !!board || queue
  const result = useRecordPages({ object: slug, ...scope, query, filters, include: slug === 'companies' ? [{ object: 'people', attribute: 'company', limit: 4 }] : undefined, sort: sort ?? (slug === 'follow_ups' ? 'action_date' : undefined), limit }, { all: everything })
  const { total, hasNextPage, isFetchingNextPage, fetchNextPage } = result
  const zone = useWorkspace()?.timezone ?? 'UTC'
  // A queue reads in the order it shows, most urgent first.
  const records = useMemo(() => queue && result.records ? byUrgency(result.records, zone).map((item) => item.record) : result.records, [queue, result.records, zone])
  // Searching narrows a view, so the queue says how much of it shows.
  const unsearched = useTool<{ total: number }>('search_records', { object: slug, ...scope, query: '', filters, limit: 1 }, { enabled: queue && !!query }).data?.total
  const { width, resize } = useColumnWidths(slug)
  const end = useRef<HTMLDivElement>(null)
  useInView(end, !everything && !!hasNextPage && !isFetchingNextPage, fetchNextPage)
  const navigate = useNavigate()
  const openRecord = (index: number) => records && navigate({ to: '/r/$recordId', params: { recordId: records[index].id } })
  const [focus, setFocus] = useListKeys(records?.map((r) => queue ? r.conversation_id ?? r.id : r.id) ?? [], queue ? { Enter: editDraft, Escape: () => setFocus(-1) } : { Enter: openRecord, o: openRecord })
  const [creating, setCreating] = useState(false)
  const rows = useRef<HTMLTableSectionElement>(null)
  useFlip(rows)
  if (!object) {
    return objects ? <Header>{slug}</Header> : <Loading />
  }
  const own = !object.standard
  // The CRM's objects show their choices first; the workspace's own tables keep the order columns were added in.
  const columns = object.attributes.filter((a) => a.slug !== 'name' && a.type !== 'markdown' && !(slug === 'follow_ups' && dateMetadata(a.slug))).sort((a, b) => (own ? 0 : Number(b.type === 'select') - Number(a.type === 'select')))
  const ownerFilter = slug === 'follow_ups' && <OwnerFilter filters={filters} onChange={(filters) => void navigate({ to: '.', search: (previous) => ({ ...previous, filters }), replace: true })} />
  const recordFilters = (title = false) => (
    <RecordFilters key={slug} object={object} filters={filters} query={q} scope={scope} selected={saved} title={title} total={total}
      onChange={(filters) => void navigate({ to: '.', search: (previous) => ({ ...previous, filters }), replace: true })}
      onApply={(saved) => void navigate({ to: '.', search: (previous) => ({ ...previous, filters: saved?.filters ?? [], q: saved?.query, saved: saved?.id }), replace: true })}
    />
  )
  const conversationFilter = conversation_id && <Button variant="ghost" aria-label="Clear conversation filter" title="Show all conversations" onClick={() => void navigate({ to: '.', search: (previous) => ({ ...previous, conversation_id: undefined }), replace: true })}>This conversation<X /></Button>
  const sortName = sort === 'updated_at' ? 'Last updated' : sort === 'name' ? 'Name' : slug === 'follow_ups' ? 'Action date' : 'Recently added'
  const sorter = (trigger: ReactNode) => (
    <Picker
      trigger={trigger}
      placeholder="Sort by…"
      options={[{ value: '', label: slug === 'follow_ups' ? 'Action date' : 'Recently added' }, { value: 'updated_at', label: 'Last updated' }, { value: 'name', label: 'Name' }]}
      selected={[sort ?? '']}
      onSelect={(value) => void navigate({ to: '.', search: { ...search, sort: value === 'name' || value === 'updated_at' ? value : undefined }, replace: true })}
      align={queue ? 'end' : 'start'}
    />
  )
  const searching = {
    type: 'search',
    autoComplete: 'off',
    value: q,
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => {
      const q = e.target.value
      void navigate({ to: '.', search: (previous) => ({ ...previous, q: q || undefined }), replace: true })
    },
    'aria-label': `Search ${object.name.toLowerCase()}`,
    'data-page-search': true,
  }
  // The queue has no page header: its own column holds what chooses its
  // records, the view as its title, the order and a search that narrows the
  // view.
  const controls = (
    <>
      <div className="flex min-w-0 flex-wrap items-center gap-1">
        <div className="-ml-2 mr-auto min-w-0">{recordFilters(true)}</div>
        {sorter(
          <Button variant="ghost" aria-label="Sort records" className="bg-list-hover hover:bg-list-active">
            {sortName}
            <ArrowUp className="text-ink-3" />
          </Button>,
        )}
      </div>
      {conversationFilter && <div>{conversationFilter}</div>}
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        {ownerFilter}
        <label className="flex h-8 min-w-0 flex-1 basis-32 items-center gap-2 rounded-[8px] bg-list-hover px-2.5 text-ink-3 transition-colors focus-within:bg-list-active">
          <Search className="size-3.5 shrink-0" />
          <input
            {...searching}
            onKeyDown={(e) => {
              e.stopPropagation()
              if (e.key === 'Escape') {
                e.currentTarget.blur()
              }
            }}
            placeholder="Search people, companies, emails"
            className="min-w-0 flex-1 bg-transparent text-[12.5px] text-ink outline-none placeholder:text-ink-3 [&::-webkit-search-cancel-button]:hidden"
          />
          {!q && <Kbd className="ml-0">/</Kbd>}
        </label>
      </div>
      {query && <p className="flex items-center justify-between px-1 text-[12px] text-ink-3">
        <span className="tabular-nums">Showing {total ?? 0} of {unsearched ?? '…'}</span>
        <button type="button" className="text-primary outline-none hover:underline focus-visible:underline" onClick={() => void navigate({ to: '.', search: (previous) => ({ ...previous, q: undefined }), replace: true })}>Clear</button>
      </p>}
    </>
  )
  return (
    <>
      {!queue && <Header className="h-auto min-h-11 flex-wrap py-2">
        <ObjectIcon slug={slug} />
        <span className="whitespace-nowrap">{object.name}</span>
        {total !== undefined && <span className="font-normal tabular-nums text-ink-3">{total}</span>}
        {conversationFilter}
        {status && (
          <div role="group" aria-label="View" className="ml-2 flex h-7 items-center rounded-full bg-list-hover p-0.5">
            <ViewButton active={!!board} label={slug === 'follow_ups' ? 'Queue' : 'Board'} onClick={() => void navigate({ to: '.', search: { ...search, view: own ? 'board' : undefined, group_by_conversation: slug === 'follow_ups' ? true : undefined }, replace: true })}>
              {slug === 'follow_ups' ? <ListChecks /> : <Kanban />}
            </ViewButton>
            <ViewButton active={!board} label="Table" onClick={() => void navigate({ to: '.', search: { ...search, view: own ? undefined : 'table', group_by_conversation: slug === 'follow_ups' ? false : undefined }, replace: true })}>
              <Table2 />
            </ViewButton>
          </div>
        )}
        <div className="ml-auto flex max-w-full flex-wrap items-center gap-1 font-normal">
          {recordFilters()}
          {ownerFilter}
          {sorter(
            <Button variant="ghost" aria-label="Sort records">
              <ArrowDownAZ />
              <span className="hidden xl:inline">{sortName}</span>
            </Button>,
          )}
          <label className="group flex h-7 shrink-0 items-center gap-1.5 rounded-[var(--radius-control)] px-2 text-ink-3 transition-colors focus-within:bg-list-hover hover:bg-list-hover">
            <Search className="size-3.5 shrink-0" />
            <input {...searching} onKeyDown={(e) => e.stopPropagation()} placeholder="Search" className="w-20 min-w-0 bg-transparent text-[12.5px] text-ink outline-none transition-[width] duration-150 placeholder:text-ink-3 focus:w-40" />
          </label>
          <Button className="ml-1" onClick={() => setCreating(true)}>
            <Plus /> New
          </Button>
        </div>
      </Header>}
      <CreateRecord object={object} open={creating} onOpenChange={setCreating} openCreated />
      {result.isError ? <EmptyState title={result.error.message} icon={<Search />} /> : board ? (
        records && <Board object={object} status={board} records={records} />
      ) : queue ? (
        <FollowUpQueue records={records ?? []} focus={focus} onFocus={setFocus} controls={controls} empty={records?.length === 0 && <Empty object={object} query={query} filtered={filters.length > 0} />} />
      ) : records?.length === 0 && (!own || query || filters.length > 0) ? (
        <Empty object={object} query={query} filtered={filters.length > 0} />
      ) : (
        <div className="scrollbar-quiet min-h-0 flex-1 overflow-auto">
          <table className="w-full table-fixed border-collapse text-[13px]">
            <colgroup>
              <col style={{ width: width('name', 280) }} />
              {slug === 'companies' && <col style={{ width: width('people', 240) }} />}
              <col style={{ width: width('updated_at', 140) }} />
              {columns.map((a) => (
                <col key={a.slug} style={{ width: width(a.slug, defaultWidth(a)) }} />
              ))}
              {own && <col style={{ width: 48 }} />}
              <col />
            </colgroup>
            <thead className="sticky top-0 z-10 bg-bg">
              <tr className="h-9 border-b border-border text-left text-[12px] text-ink-3">
                <th className="sticky left-0 z-20 bg-bg px-4 font-medium">
                  {own ? <ColumnHeader object={object} attribute={object.attributes.find((a) => a.slug === 'name')!} /> : slug === 'companies' ? 'Company' : 'Name'}
                  <Resizer onPointerDown={resize('name', 280)} />
                </th>
                {slug === 'companies' && (
                  <th className="relative px-3 font-medium">
                    People
                    <Resizer onPointerDown={resize('people', 240)} />
                  </th>
                )}
                <th className="relative px-3 font-medium">
                  Last updated
                  <Resizer onPointerDown={resize('updated_at', 140)} />
                </th>
                {columns.map((a) => (
                  <th key={a.slug} className="relative px-3 font-medium">
                    {own ? <ColumnHeader object={object} attribute={a} /> : <span className="block truncate">{a.name}</span>}
                    <Resizer onPointerDown={resize(a.slug, defaultWidth(a))} />
                  </th>
                ))}
                {own && (
                  <th className="px-1">
                    <AddColumn object={object} objects={objects ?? []}>
                      <Button variant="ghost" size="icon-sm" aria-label="Add column">
                        <Plus />
                      </Button>
                    </AddColumn>
                  </th>
                )}
                <th />
              </tr>
            </thead>
            <tbody ref={rows}>
              {records?.map((r, index) => (
                <RecordMenu key={r.id} object={object} record={r}>
                  <tr
                    data-row={index}
                    data-flip={r.id}
                    onClick={() => openRecord(index)}
                    className={cn('group h-10 cursor-default border-b border-border/50 hover:bg-list-hover data-[state=open]:bg-list-hover', focus === index && 'bg-list-hover')}
                  >
                    <td className={cn('sticky left-0 z-10 bg-bg px-4 group-hover:bg-list-hover group-data-[state=open]:bg-list-hover', focus === index && 'bg-list-hover')}>
                      <Link to="/r/$recordId" params={{ recordId: r.id }} onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()} className="flex min-w-0 items-center gap-2.5 font-medium text-ink">
                        <RecordIcon object={slug} name={recordName(r)} photo={r.photo} icon={pageIcon(r)} />
                        <span className="min-w-0 truncate">{recordName(r)}</span>
                      </Link>
                    </td>
                    {slug === 'companies' && <td className="overflow-hidden px-3 text-ink-2"><CompanyPeople company={r} /></td>}
                    <td className="px-3 text-[12px] text-ink-3"><UpdatedAt at={r.updated_at} /></td>
                    {columns.map((a) => (
                      <td key={a.slug} className="overflow-hidden px-3 py-1 text-ink-2">
                        {own ? (
                          <div onClick={(e) => e.stopPropagation()}>
                            <Field record={r} attribute={a} limit={5} />
                          </div>
                        ) : a.type === 'select' ? (
                          <div onClick={(e) => e.stopPropagation()}>
                            <SelectField record={r} attribute={a} />
                          </div>
                        ) : (
                          cell(r, a)
                        )}
                      </td>
                    ))}
                    {own && <td />}
                    <td />
                  </tr>
                </RecordMenu>
              ))}
            </tbody>
          </table>
          <div ref={end} aria-hidden className="h-px" />
        </div>
      )}
    </>
  )
}

// defaultWidth fits a column's usual content until someone drags it wider.
const defaultWidth = (attribute: Attribute) => (attribute.multi || attribute.type === 'reference' ? 280 : attribute.type === 'select' || attribute.type === 'status' ? 200 : 180)

// Resizer is the edge of a table header that drags its column's width.
function Resizer({ onPointerDown }: { onPointerDown: (e: React.PointerEvent) => void }) {
  return <span aria-hidden onPointerDown={onPointerDown} onClick={(e) => e.stopPropagation()} className="absolute inset-y-0 right-0 w-1.5 cursor-col-resize touch-none transition-colors hover:bg-primary/40" />
}

function ViewButton({ active, label, onClick, children }: { active: boolean; label: string; onClick: () => void; children: ReactNode }) {
  return (
    <Button variant="ghost" size="icon-sm" aria-label={label} aria-pressed={active} title={label} onClick={onClick} className={active ? 'bg-raised text-ink' : undefined}>
      {children}
    </Button>
  )
}

function cell(record: CrmRecord, attribute: Attribute) {
  const values = valuesOf(record, attribute.slug).map(valueText)
  if (attribute.type === 'status' && values[0]) {
    return <Stage stage={values[0]} />
  }
  if (attribute.type === 'domain' || attribute.type === 'url') {
    return <div className="flex flex-col gap-1">{values.map((value) => <ExternalLink key={value} href={attribute.type === 'domain' ? `https://${value}` : value} className="block truncate">{value}</ExternalLink>)}</div>
  }
  if (attribute.type === 'datetime') {
    return values.map((value) => <DateLabel key={value} value={value} suggested={attribute.slug === 'action_date' && record.values.action_date_basis === 'Suggested'} />)
  }
  if (attribute.type === 'date') {
    return values.map(formatDay).join(', ')
  }
  if (attribute.type === 'checkbox') {
    return values[0] === 'true' ? 'Yes' : ''
  }
  if (attribute.type === 'number') {
    return values.map((v) => formatNumber(v, attribute.slug)).join(', ')
  }
  return <span className="block truncate" title={values.join(', ')}>{values.join(', ')}</span>
}

function Empty({ object, query, filtered }: { object: CrmObject; query: string; filtered: boolean }) {
  const mail = useMail()
  const plural = object.name.toLowerCase()
  if (query) {
    return <EmptyState title={`No ${plural} match “${query}”`} icon={<Search />} />
  }
  if (filtered) {
    return <EmptyState title={`No ${plural} match this view`} icon={<Search />} />
  }
  const synced = object.slug in arrivals
  return (
    <EmptyState title={`No ${plural} yet`} icon={<ObjectIcon slug={object.slug} />} action={synced && mail === 'none' && <ConnectGoogle />}>
      {arrivals[object.slug] ?? 'Records you add with New, or that an agent creates, appear here.'}
    </EmptyState>
  )
}
