import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { ArrowDownAZ, Kanban, ListChecks, Plus, Search, Table2, X } from 'lucide-react'
import { type ReactNode, useMemo, useRef, useState } from 'react'
import { Board } from '@/components/board'
import { CompanyPeople } from '@/components/company-people'
import { editDraft } from '@/components/follow-up'
import { byUrgency, FollowUpQueue } from '@/components/follow-ups'
import { Stage } from '@/components/stage'
import { Button } from '@jaz/ui/button'
import { Header, Loading } from '@/components/controls'
import { AddColumn, ColumnHeader } from '@/components/columns'
import { CreateRecord } from '@/components/create-record'
import { Field } from '@/components/fields'
import { valueLinks } from '@/components/external-link'
import { ConnectGoogle, EmptyState } from '@/components/empty-state'
import { ObjectIcon, RecordIcon } from '@/components/icons'
import { Picker } from '@/components/picker'
import { OwnerFilter } from '@/components/owner-filter'
import { RecordFilters } from '@/components/record-filters'
import { RecordMenu } from '@/components/record-menu'
import { NavButton } from '@/components/nav-drawer'
import { SelectField } from '@/components/select-field'
import { UpdatedAt } from '@/components/updated-at'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { pageIcon } from '@/lib/pages'
import { formatDay, formatNumber } from '@/lib/format'
import { useDebounced, useFlip, useInView, useListKeys, usePhone } from '@/lib/hooks'
import { DateLabel, dateMetadata } from '@/components/date-field'
import { useObjects, useRecordPages, useTool, useWorkspace } from '@/lib/queries'
import { useColumnWidths } from '@/lib/use-column-widths'
import { filterKeys, validateRecordSearch } from '@/lib/record-search'
import { statusOf } from '@/lib/stages'
import { useMail } from '@/lib/sync'
import type { Attribute, CrmObject, CrmRecord, RecordFilter, SavedFilter } from '@/lib/types'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_app/o/$object')({
  validateSearch: validateRecordSearch,
  // Filters read as their own keys in the URL, such as ?tags=Lead.
  search: { middlewares: [({ search, next }) => {
    const { filters, ...rest } = next(search)
    return { ...rest, ...filterKeys(filters) }
  }] },
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
  const defaults: RecordFilter[] = slug === 'follow_ups' ? [
    { attribute: 'status', operator: 'is', value: 'Open' },
    ...(me ? [{ attribute: 'owner', operator: 'is' as const, value: me.email }] : []),
  ] : []
  const { sort, view, saved, limit = 100, conversation_id } = search
  // A saved view's link shows the view; changing its filters or search edits it.
  const views = useTool<{ filters: SavedFilter[] }>('list_saved_filters', { object: slug }, { refetchInterval: 5000 })
  const chosen = views.data?.filters.find((f) => f.id === saved)
  // A key naming no attribute, such as a link's tracking parameter, neither
  // filters nor replaces a saved view.
  const linked = search.filters?.filter((f) => object?.attributes.some((a) => a.slug === f.attribute))
  const filters = (search.filters?.length && !linked?.length ? undefined : linked) ?? chosen?.filters ?? defaults
  const q = search.q ?? chosen?.query ?? ''
  // A list's defaults stay out of its URL; a saved view keeps every change.
  const urlFilters = (next: RecordFilter[]) => (!saved && JSON.stringify(next) === JSON.stringify(defaults) ? undefined : next)
  const query = useDebounced(q.trim())
  const queue = slug === 'follow_ups' && (view ? view !== 'table' : search.group_by_conversation !== false)
  const scope = { group_by_conversation: slug === 'follow_ups' ? queue : undefined, conversation_id }
  const status = object && statusOf(object.attributes)
  // The workspace's own tables open as tables, the CRM's pipelines as boards.
  const board = slug !== 'follow_ups' && (view ?? (object?.standard ? 'board' : 'table')) === 'board' ? status : undefined
  const everything = !!board || queue
  const result = useRecordPages({ object: slug, ...scope, query, filters, include: slug === 'companies' ? [{ object: 'people', attribute: 'company', limit: 4 }] : undefined, sort: sort ?? (slug === 'follow_ups' ? 'action_date' : undefined), limit }, { all: everything, enabled: !!object && (!saved || !!views.data) })
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
  const list = useRef<HTMLUListElement>(null)
  useFlip(list)
  const phone = usePhone()
  if (!object) {
    return objects ? <Header><NavButton />{slug}</Header> : <Loading />
  }
  const own = !object.standard
  // The CRM's objects show their choices first; the workspace's own tables keep the order columns were added in.
  const columns = object.attributes.filter((a) => a.slug !== 'name' && a.type !== 'markdown' && !(slug === 'follow_ups' && dateMetadata(a.slug))).sort((a, b) => (own ? 0 : Number(b.type === 'select') - Number(a.type === 'select')))
  const ownerFilter = slug === 'follow_ups' && <OwnerFilter filters={filters} onChange={(filters) => void navigate({ to: '.', search: (previous) => ({ ...previous, filters: urlFilters(filters) }), replace: true })} />
  const recordFilters = (title = false) => (
    <RecordFilters key={slug} object={object} filters={filters} defaults={defaults} query={q} scope={scope} selected={saved} title={title} total={total}
      onChange={(filters) => void navigate({ to: '.', search: (previous) => ({ ...previous, filters: urlFilters(filters) }), replace: true })}
      onApply={(picked) => void navigate({ to: '.', search: (previous) => ({ ...previous, filters: picked || !defaults.length ? undefined : [], q: undefined, saved: picked?.id }), replace: true })}
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
    />
  )
  // On a phone an open search takes the header row; the title and the
  // other controls step aside until it closes.
  const phoneSearch = q ? 'max-md:hidden' : 'max-md:group-has-[[data-page-search]:focus]/header:hidden'
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
  // The queue has no page header: one row of its column holds the view as
  // its title, a filter, whose follow-ups show and a search that narrows the
  // view, which opens from its icon or with /.
  const controls = (
    <>
      <div className="flex min-w-0 items-center gap-1">
        <NavButton />
        <div className="mr-auto min-w-0 md:-ml-2">{recordFilters(true)}</div>
        {ownerFilter}
        <label title="Search" className={cn('flex h-7 min-w-7 shrink-0 cursor-text items-center gap-2 rounded-[var(--radius-control)] px-1.5 text-ink-2 transition-colors focus-within:bg-list-hover hover:bg-list-hover', q && 'bg-list-hover')}>
          <Search className="size-4 shrink-0" />
          <input
            {...searching}
            onKeyDown={(e) => {
              e.stopPropagation()
              if (e.key === 'Escape') {
                e.currentTarget.blur()
              }
            }}
            placeholder="Search"
            className={cn('w-0 min-w-0 bg-transparent text-[12.5px] text-ink outline-none transition-[width] duration-150 placeholder:text-ink-3 focus:w-40 motion-reduce:transition-none [&::-webkit-search-cancel-button]:hidden', q && 'w-40')}
          />
        </label>
      </div>
      {conversationFilter && <div>{conversationFilter}</div>}
      {query && <p className="flex items-center justify-between px-1 text-[12px] text-ink-3">
        <span className="tabular-nums">Showing {total ?? 0} of {unsearched ?? '…'}</span>
        <button type="button" className="text-primary outline-none hover:underline focus-visible:underline" onClick={() => void navigate({ to: '.', search: (previous) => ({ ...previous, q: undefined }), replace: true })}>Clear</button>
      </p>}
    </>
  )
  return (
    <>
      {!queue && <Header className="group/header h-auto min-h-11 py-2 md:flex-wrap">
        <NavButton />
        <ObjectIcon slug={slug} />
        <span className={cn('whitespace-nowrap max-md:min-w-0 max-md:truncate', phoneSearch)}>{object.name}</span>
        {total !== undefined && <span className={cn('font-normal tabular-nums text-ink-3', phoneSearch)}>{total}</span>}
        {conversationFilter}
        {status && (
          <div role="group" aria-label="View" className={cn('ml-2 flex h-7 items-center rounded-full bg-list-hover p-0.5', phoneSearch)}>
            <ViewButton active={!!board} label={slug === 'follow_ups' ? 'Queue' : 'Board'} onClick={() => void navigate({ to: '.', search: { ...search, view: own ? 'board' : undefined, group_by_conversation: slug === 'follow_ups' ? true : undefined }, replace: true })}>
              {slug === 'follow_ups' ? <ListChecks /> : <Kanban />}
            </ViewButton>
            <ViewButton active={!board} label="Table" onClick={() => void navigate({ to: '.', search: { ...search, view: own ? undefined : 'table', group_by_conversation: slug === 'follow_ups' ? false : undefined }, replace: true })}>
              <Table2 />
            </ViewButton>
          </div>
        )}
        <div className={cn('ml-auto flex max-w-full items-center gap-1 font-normal md:flex-wrap', q ? 'max-md:flex-1' : 'max-md:group-has-[[data-page-search]:focus]/header:flex-1')}>
          <span className={cn('contents', phoneSearch)}>
            {recordFilters()}
            {ownerFilter}
            {sorter(
              <Button variant="ghost" aria-label="Sort records" className="max-md:px-1.5">
                <ArrowDownAZ />
                <span className="hidden xl:inline">{sortName}</span>
              </Button>,
            )}
          </span>
          <label className={cn('group flex h-7 shrink-0 items-center gap-1.5 rounded-[var(--radius-control)] px-2 text-ink-3 transition-colors focus-within:bg-list-hover hover:bg-list-hover', q ? 'max-md:flex-1' : 'max-md:group-has-[[data-page-search]:focus]/header:flex-1')}>
            <Search className="size-3.5 shrink-0" />
            <input {...searching} onKeyDown={(e) => e.stopPropagation()} placeholder="Search" className={cn('w-20 min-w-0 bg-transparent text-[12.5px] text-ink outline-none transition-[width] duration-150 placeholder:text-ink-3 focus:w-40 max-md:w-0 max-md:focus:w-full', q && 'max-md:w-full')} />
          </label>
          <Button className="ml-1 max-md:w-7 max-md:px-0" onClick={() => setCreating(true)}>
            <Plus /> <span className="max-md:sr-only">New</span>
          </Button>
        </div>
      </Header>}
      <CreateRecord object={object} open={creating} onOpenChange={setCreating} openCreated />
      {result.isError ? <EmptyState title={result.error.message} icon={<Search />} /> : board ? (
        records && <Board object={object} status={board} records={records} />
      ) : queue ? (
        <FollowUpQueue records={records ?? []} focus={focus} onFocus={setFocus} controls={controls} empty={records?.length === 0 && <Empty object={object} query={query} filtered={filters.length > 0} />} />
      ) : records?.length === 0 && (!own || query || filters.length > 0 || phone) ? (
        <Empty object={object} query={query} filtered={filters.length > 0} />
      ) : phone ? (
        <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
          <ul ref={list} className="py-1">
            {records?.map((r, index) => (
              <li key={r.id} data-flip={r.id}>
                <RecordMenu object={object} record={r}>
                  <Link to="/r/$recordId" params={{ recordId: r.id }} data-row={index} className={cn('flex h-13 items-center gap-3 px-4 outline-none hover:bg-list-hover focus-visible:bg-list-hover data-[state=open]:bg-list-hover', focus === index && 'bg-list-hover')}>
                    <RecordIcon object={slug} name={recordName(r)} photo={r.photo} icon={pageIcon(r)} size={28} />
                    <span className="flex min-w-0 flex-col">
                      <span className="truncate font-medium text-ink">{recordName(r)}</span>
                      <span className="truncate text-[12px] text-ink-3">{glance(r, object)}</span>
                    </span>
                  </Link>
                </RecordMenu>
              </li>
            ))}
          </ul>
          <div ref={end} aria-hidden className="h-px" />
        </div>
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

// glance is the one value a record's row shows under its name at phone
// width: its stage, the record it belongs to, or what identifies it.
function glance(record: CrmRecord, object: CrmObject) {
  const held = (a: Attribute) => valuesOf(record, a.slug).length > 0
  const attribute = object.attributes.find((a) => a.type === 'status' && held(a))
    ?? object.attributes.find((a) => a.type === 'reference' && !a.multi && held(a))
    ?? object.attributes.find((a) => a.unique && held(a))
  if (!attribute) {
    return null
  }
  const text = valueText(valuesOf(record, attribute.slug)[0])
  return attribute.type === 'status' ? <Stage stage={text} /> : text
}

function cell(record: CrmRecord, attribute: Attribute) {
  const values = valuesOf(record, attribute.slug).map(valueText)
  if (attribute.type === 'status' && values[0]) {
    return <Stage stage={values[0]} />
  }
  const ValueLink = valueLinks[attribute.type]
  if (ValueLink) {
    return <div className="flex flex-col gap-1">{values.map((value) => <ValueLink key={value} value={value} className="block truncate" />)}</div>
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
