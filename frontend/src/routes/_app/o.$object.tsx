import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { ArrowDownAZ, ChevronRight, Kanban, ListChecks, Plus, Search, Table2, X } from 'lucide-react'
import { type ReactNode, useMemo, useRef, useState } from 'react'
import { RecordCommands } from '@/components/record-commands'
import { useShortcutActions } from '@/lib/shortcuts'
import { Board } from '@/components/board'
import { RecordTable } from '@/components/record-table'
import { DatabaseHeader } from '@/components/database-header'
import { editDraft } from '@/components/follow-up'
import { byUrgency, FollowUpQueue } from '@/components/follow-ups'
import { Stage } from '@/components/stage'
import { Button } from '@jaz/ui/button'
import { Header, Loading } from '@/components/controls'
import { CreateRecord } from '@/components/create-record'
import { ConnectGoogle, EmptyState } from '@/components/empty-state'
import { ObjectIcon, RecordIcon } from '@/components/icons'
import { Picker } from '@/components/picker'
import { OwnerFilter } from '@/components/owner-filter'
import { RecordFilters } from '@/components/record-filters'
import { RecordMenu } from '@/components/record-menu'
import { NavButton } from '@/components/nav-drawer'
import { ListMotion } from '@/components/list-motion'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { pageIcon, path, usePages } from '@/lib/pages'
import { useDebounced, useInView, useListKeys, usePhone } from '@/lib/hooks'
import { useObjects, useRecordPages, useTool, useWorkspace } from '@/lib/queries'
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
  const pages = usePages()
  const ancestors = object?.parent && pages ? path(pages, object.parent) : []
  const me = useWorkspace()?.members?.find((m) => m.is_me)
  const search = Route.useSearch()
  const defaults: RecordFilter[] = slug === 'follow_ups' ? [
    { attribute: 'status', operator: 'is', value: 'Open' },
    ...(me ? [{ attribute: 'owner', operator: 'is' as const, value: me.email }] : []),
  ] : []
  const { sort, view, saved, limit = 100, conversation_id } = search
  // A saved view's link shows the view; changing its filters or search edits it.
  const views = useTool<{ filters: SavedFilter[] }>('list_saved_filters', { object: slug })
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
  const result = useRecordPages({ object: slug, ...scope, query, filters, include: slug === 'companies' ? [{ object: 'people', attribute: 'company', limit: 20 }] : undefined, sort: sort ?? (slug === 'follow_ups' ? 'action_date' : undefined), limit }, { all: everything, enabled: !!object && (!saved || !!views.data) })
  const { total, hasNextPage, isFetchingNextPage, fetchNextPage } = result
  const zone = useWorkspace()?.timezone ?? 'UTC'
  // A queue reads in the order it shows, most urgent first.
  const ordered = useMemo(() => queue && result.records ? byUrgency(result.records, zone).map((item) => item.record) : result.records, [queue, result.records, zone])
  const records = board && ordered ? (board.options ?? []).flatMap((stage) => ordered.filter((record) => valueText(valuesOf(record, board.slug)[0] ?? '') === stage)) : ordered
  // Searching narrows a view, so the queue says how much of it shows.
  const unsearched = useTool<{ total: number }>('search_records', { object: slug, ...scope, query: '', filters, limit: 1 }, { enabled: queue && !!query }).data?.total
  const end = useRef<HTMLDivElement>(null)
  useInView(end, !everything && !!hasNextPage && !isFetchingNextPage, fetchNextPage)
  const navigate = useNavigate()
  const openRecord = (index: number) => records && navigate({ to: '/r/$recordId', params: { recordId: records[index].id } })
  const [focus, setFocus] = useListKeys(records?.map((r) => queue ? r.conversation_id ?? r.id : r.id) ?? [], queue ? { open: editDraft, back: () => setFocus(-1) } : { open: openRecord, back: () => setFocus(-1) })
  const [creating, setCreating] = useState(false)
  const phone = usePhone()
  const across = (delta: number) => {
    if (!board || !records?.length) {
      return
    }
    const groups = (board.options ?? []).map((stage) => records.filter((record) => valueText(valuesOf(record, board.slug)[0] ?? '') === stage)).filter((group) => group.length)
    const column = groups.findIndex((group) => group.includes(records[focus]))
    const row = groups[column]?.indexOf(records[focus]) ?? 0
    const next = groups[column + delta]
    if (next) {
      setFocus(records.indexOf(next[Math.min(Math.max(0, row), next.length - 1)]))
    }
  }
  useShortcutActions({
    ...(board && { left: () => across(-1), right: () => across(1) }),
    create: () => setCreating(true),
    ...(status && {
      layout: () => { void navigate({ to: '.', search: { ...search, view: board || queue ? 'table' : 'board', group_by_conversation: slug === 'follow_ups' ? !queue : undefined }, replace: true }) },
      board: () => { void navigate({ to: '.', search: { ...search, view: 'board', group_by_conversation: slug === 'follow_ups' ? true : undefined }, replace: true }) },
    }),
    all: () => { void navigate({ to: '.', search: { view: search.view, filters: [] }, replace: true }) },
    clearFilter: () => { void navigate({ to: '.', search: { ...search, saved: undefined, filters: urlFilters(filters.slice(0, -1)) }, replace: true }) },
    clearFilters: () => { void navigate({ to: '.', search: { ...search, q: undefined, saved: undefined, filters: [] }, replace: true }) },
  })
  if (!object) {
    return objects ? <Header><NavButton />{slug}</Header> : <Loading />
  }
  const own = !object.standard
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
  const tableToolbar = (
    <>
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
      <div className={cn('flex min-w-0 max-w-full items-center gap-1 font-normal md:flex-wrap', own ? 'flex-1' : 'ml-auto', q ? 'max-md:flex-1' : 'max-md:group-has-[[data-page-search]:focus]/header:flex-1')}>
        <span className={cn('flex min-w-0 items-center gap-1', own && 'mr-auto', phoneSearch)}>
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
    </>
  )
  return (
    <>
      {records?.[focus] && <RecordCommands key={records[focus].id} record={records[focus]} object={object} />}
      {!queue && <Header className="group/header h-auto min-h-11 py-2 md:flex-wrap">
        <NavButton />
        {ancestors.length > 0 && <nav aria-label="Parent pages" className={cn('flex min-w-0 items-center gap-1.5', phoneSearch)}>
          {ancestors.map((page) => <span key={page.id} className="flex min-w-0 items-center gap-1.5">
            <Link to="/r/$recordId" params={{ recordId: page.id }} title={page.name} className="truncate text-ink-2 hover:text-ink">{page.name}</Link>
            <ChevronRight aria-hidden className="size-3 shrink-0 text-ink-3" />
          </span>)}
        </nav>}
        <ObjectIcon slug={slug} icon={object.icon} />
        <span className={cn('whitespace-nowrap max-md:min-w-0 max-md:truncate', phoneSearch)}>{object.name}</span>
        {!own && total !== undefined && <span className={cn('font-normal tabular-nums text-ink-3', phoneSearch)}>{total}</span>}
        {conversationFilter}
        {!own && tableToolbar}
      </Header>}
      {own && <>
        <DatabaseHeader key={object.slug} object={object} />
        <div className="group/header flex min-h-11 shrink-0 items-center gap-2 border-b border-border px-5 py-2 md:px-8">{tableToolbar}</div>
      </>}
      <CreateRecord object={object} open={creating} onOpenChange={setCreating} openCreated />
      <ListMotion className="flex min-h-0 flex-1 flex-col" scope={JSON.stringify([query, filters, view, sort, conversation_id])}>
        {result.isError ? <EmptyState title={result.error.message} icon={<Search />} /> : board ? (
          records && <Board object={object} status={board} records={records} focus={focus} onFocus={setFocus} />
        ) : queue ? (
          <FollowUpQueue records={records ?? []} focus={focus} onFocus={setFocus} controls={controls} empty={records?.length === 0 && <Empty object={object} query={query} filtered={filters.length > 0} />} />
        ) : records?.length === 0 && (!own || query || filters.length > 0 || phone) ? (
          <Empty object={object} query={query} filtered={filters.length > 0} />
        ) : phone ? (
          <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
            <ul className="py-1">
              {records?.map((r, index) => (
                <li key={r.id} data-flip={r.id}>
                  <RecordMenu object={object} record={r}>
                    <Link to="/r/$recordId" params={{ recordId: r.id }} data-row={index} onPointerEnter={() => setFocus(index)} className={cn('flex h-13 items-center gap-3 px-4 outline-none hover:bg-list-hover focus-visible:bg-list-hover data-[state=open]:bg-list-hover', focus === index && 'bg-list-hover')}>
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
          <div className={cn("scrollbar-quiet min-h-0 flex-1 overflow-auto", own && "px-5 md:px-8")}>
            <RecordTable object={object} objects={objects ?? []} records={records ?? []} focus={focus} onFocus={setFocus} openRecord={openRecord} />
            <div ref={end} aria-hidden className="h-px" />
          </div>
        )}
      </ListMotion>
    </>
  )
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
